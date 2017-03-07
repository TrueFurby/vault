package dbs

import (
	"fmt"
	"strings"

	"github.com/gocql/gocql"
	"github.com/hashicorp/vault/helper/strutil"
)

const (
	defaultCreationCQL = `CREATE USER '{{username}}' WITH PASSWORD '{{password}}' NOSUPERUSER;`
	defaultRollbackCQL = `DROP USER '{{username}}';`
)

type Cassandra struct {
	// Session is goroutine safe, however, since we reinitialize
	// it when connection info changes, we want to make sure we
	// can close it and use a new connection; hence the lock
	ConnectionProducer
	CredentialsProducer
}

func (c *Cassandra) Type() string {
	return cassandraTypeName
}

func (c *Cassandra) getConnection() (*gocql.Session, error) {
	session, err := c.connection()
	if err != nil {
		return nil, err
	}

	return session.(*gocql.Session), nil
}

func (c *Cassandra) CreateUser(createStmts, rollbackStmts, username, password, expiration string) error {
	// Grab the lock
	c.Lock()
	defer c.Unlock()

	// Get the connection
	session, err := c.getConnection()
	if err != nil {
		return err
	}

	// Execute each query
	for _, query := range strutil.ParseArbitraryStringSlice(createStmts, ";") {
		query = strings.TrimSpace(query)
		if len(query) == 0 {
			continue
		}

		err = session.Query(queryHelper(query, map[string]string{
			"username": username,
			"password": password,
		})).Exec()
		if err != nil {
			for _, query := range strutil.ParseArbitraryStringSlice(rollbackStmts, ";") {
				query = strings.TrimSpace(query)
				if len(query) == 0 {
					continue
				}

				session.Query(queryHelper(query, map[string]string{
					"username": username,
					"password": password,
				})).Exec()
			}
			return err
		}
	}

	return nil
}

func (c *Cassandra) RenewUser(username, expiration string) error {
	// NOOP
	return nil
}

func (c *Cassandra) RevokeUser(username, revocationStmts string) error {
	// Grab the lock
	c.Lock()
	defer c.Unlock()

	session, err := c.getConnection()
	if err != nil {
		return err
	}

	err = session.Query(fmt.Sprintf("DROP USER '%s'", username)).Exec()
	if err != nil {
		return fmt.Errorf("error removing user %s", username)
	}

	return nil
}
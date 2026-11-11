## Migrations

If a database schema file has been added, make sure you are in sql/schema, and run:

`goose postgres $cons up`

To rollback, the command is the same other than the last parameter is down. 

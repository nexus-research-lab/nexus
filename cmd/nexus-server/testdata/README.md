# Previous-release upgrade fixture

`v0.2.5.db.gz` contains synthetic Room, Conversation and Goal records, never user data.
The schema was generated from tag `v0.2.5`, commit
`55b317afa56c6d39e65e1e550ab167909b5545fd`, using its complete SQLite migration chain
(version 158). It is intentionally frozen: do not regenerate it from current migrations
when a migration test fails.

To reproduce, export that tag's `db/migrations/sqlite` directory using `git archive`,
run Goose v3.27.0 against a new SQLite database, apply `v0.2.5-seed.sql`, close the
connection and gzip the database. Goose timestamps and SQLite file bytes need not
be identical; the seed SQL records the exact synthetic business data.

`TestPreviousReleaseDatabaseUpgrade` decompresses into a temporary directory, runs
the production startup migration function twice, then verifies Room/Conversation/Goal
data and foreign keys. This covers database upgrades, not runtime transcripts or
native sandbox upgrades.

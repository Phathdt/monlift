# Monlift

A MongoDB migration tool written in Go, inspired by Goose and Prisma. This tool helps you manage your MongoDB database schema migrations.

## Features

- Create and manage MongoDB migrations
- Run migrations up and down
- Track migration status
- Support for various MongoDB operations:
  - Create/drop collections
  - Create/drop indexes
  - Insert/update/delete documents
  - Aggregate operations

## Installation

```bash
go install github.com/phathdt/monlift
```

## Configuration

Create a `.env` file in your project root:

```env
DB_URI=mongodb://localhost:27017/test-migrate
```

## Usage

### Create a new migration

```bash
monlift create add_users_collection
```

This will create two files in the `migrations` directory:
- `YYYYMMDDHHMMSS_add_users_collection.up.js`
- `YYYYMMDDHHMMSS_add_users_collection.down.js`

### Run migrations

Run all pending migrations:

```bash
monlift up
```

Rollback the last migration:

```bash
monlift down
```

### Check migration status

```bash
monlift status
```

Output example:
```
Version    Status    Applied At
-------    ------    ----------
20240101   Applied   2024-01-01 10:00:00
20240102   Pending
```

## Migration Scripts

Migration scripts use MongoDB shell syntax. Here are some examples:

### Create Collection

```javascript
db.createCollection("users")
```

### Create Index

```javascript
db.users.createIndex({"email": 1}, {"unique": true})
```

### Insert Documents

```javascript
db.users.insertOne({"name": "John", "email": "john@example.com"})
db.users.insertMany([{"name": "Alice"}, {"name": "Bob"}])
```

### Update Documents

```javascript
db.users.updateMany({"name": "John"}, {"$set": {"age": 30}})
db.users.updateOne({"name": "John"}, {"$set": {"age": 30}})
```

### Delete Documents

```javascript
db.users.deleteMany({"name": "John"})
db.users.deleteOne({"name": "John"})
```

### Drop Collection

```javascript
db.users.drop()
```

### Aggregate

```javascript
db.users.aggregate([
  {"$match": {"age": {"$gt": 18}}},
  {"$project": {"name": 1}}
])
```

## Development

### Prerequisites

- Go 1.16 or later
- MongoDB 4.4 or later

### Building from source

```bash
git clone https://github.com/phathdt/monlift.git
cd monlift
make build
```

### Running tests

```bash
make test
```

## License

MIT

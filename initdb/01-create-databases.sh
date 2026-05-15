#!/bin/bash
set -e

# Create alipay-vmq database and user
if [ -n "$ALIPAY_VMQ_DB_NAME" ] && [ -n "$ALIPAY_VMQ_DB_USER" ] && [ -n "$ALIPAY_VMQ_DB_PASSWORD" ]; then
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    SELECT 'CREATE DATABASE ${ALIPAY_VMQ_DB_NAME}'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${ALIPAY_VMQ_DB_NAME}')\gexec

    DO \$\$
    BEGIN
      IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${ALIPAY_VMQ_DB_USER}') THEN
        CREATE ROLE ${ALIPAY_VMQ_DB_USER} WITH LOGIN PASSWORD '${ALIPAY_VMQ_DB_PASSWORD}';
      END IF;
    END
    \$\$;

    GRANT ALL PRIVILEGES ON DATABASE ${ALIPAY_VMQ_DB_NAME} TO ${ALIPAY_VMQ_DB_USER};
    ALTER DATABASE ${ALIPAY_VMQ_DB_NAME} OWNER TO ${ALIPAY_VMQ_DB_USER};
EOSQL
  echo "alipay-vmq database initialized: ${ALIPAY_VMQ_DB_NAME}"
fi

# Create VMQ database and user (if configured)
if [ -n "$VMQ_DB_NAME" ] && [ -n "$VMQ_DB_USER" ] && [ -n "$VMQ_DB_PASSWORD" ]; then
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    SELECT 'CREATE DATABASE ${VMQ_DB_NAME}'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${VMQ_DB_NAME}')\gexec

    DO \$\$
    BEGIN
      IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${VMQ_DB_USER}') THEN
        CREATE ROLE ${VMQ_DB_USER} WITH LOGIN PASSWORD '${VMQ_DB_PASSWORD}';
      END IF;
    END
    \$\$;

    GRANT ALL PRIVILEGES ON DATABASE ${VMQ_DB_NAME} TO ${VMQ_DB_USER};
    ALTER DATABASE ${VMQ_DB_NAME} OWNER TO ${VMQ_DB_USER};
EOSQL
  echo "VMQ database initialized: ${VMQ_DB_NAME}"
fi

# Create Dujiao database and user (if configured)
if [ -n "$DUJIAO_DB_NAME" ] && [ -n "$DUJIAO_DB_USER" ] && [ -n "$DUJIAO_DB_PASSWORD" ]; then
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    SELECT 'CREATE DATABASE ${DUJIAO_DB_NAME}'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DUJIAO_DB_NAME}')\gexec

    DO \$\$
    BEGIN
      IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DUJIAO_DB_USER}') THEN
        CREATE ROLE ${DUJIAO_DB_USER} WITH LOGIN PASSWORD '${DUJIAO_DB_PASSWORD}';
      END IF;
    END
    \$\$;

    GRANT ALL PRIVILEGES ON DATABASE ${DUJIAO_DB_NAME} TO ${DUJIAO_DB_USER};
    ALTER DATABASE ${DUJIAO_DB_NAME} OWNER TO ${DUJIAO_DB_USER};
EOSQL
  echo "Dujiao database initialized: ${DUJIAO_DB_NAME}"
fi

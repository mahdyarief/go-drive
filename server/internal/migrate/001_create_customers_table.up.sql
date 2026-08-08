-- +migrate Up
DO $$ 
DECLARE
  schema_name TEXT;
BEGIN
  FOR schema_name IN 
    SELECT schema_name 
    FROM information_schema.schemata 
    WHERE schema_name LIKE 'tenant%' 
  LOOP
    EXECUTE format('
      CREATE TABLE IF NOT EXISTS %I.customers (
        id         SERIAL PRIMARY KEY,
        name       TEXT NOT NULL,
        phone      TEXT,
        email      TEXT,
        created_at TIMESTAMPTZ DEFAULT NOW(),
        updated_at TIMESTAMPTZ DEFAULT NOW()
      );

      CREATE INDEX IF NOT EXISTS idx_%I_customers_name ON %I.customers USING gin (name gin_trgm_ops);
      CREATE INDEX IF NOT EXISTS idx_%I_customers_phone ON %I.customers (phone);
    ', schema_name, schema_name, schema_name, schema_name, schema_name);
  END LOOP;
END $$;
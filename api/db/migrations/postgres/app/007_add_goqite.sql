CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION update_goqite_timestamp()
RETURNS trigger AS $$
BEGIN
   NEW.updated = now();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS goqite (
  id text PRIMARY KEY DEFAULT ('m_' || encode(gen_random_bytes(16), 'hex')),
  created timestamptz NOT NULL DEFAULT now(),
  updated timestamptz NOT NULL DEFAULT now(),
  queue text NOT NULL,
  body bytea NOT NULL,
  timeout timestamptz NOT NULL DEFAULT now(),
  received integer NOT NULL DEFAULT 0,
  priority integer NOT NULL DEFAULT 0
);

DROP TRIGGER IF EXISTS goqite_updated_timestamp ON goqite;
CREATE TRIGGER goqite_updated_timestamp
BEFORE UPDATE ON goqite
FOR EACH ROW EXECUTE PROCEDURE update_goqite_timestamp();

CREATE INDEX IF NOT EXISTS goqite_queue_priority_created_idx ON goqite (queue, priority DESC, created);

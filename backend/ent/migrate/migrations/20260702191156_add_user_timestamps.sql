-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP, ADD COLUMN "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

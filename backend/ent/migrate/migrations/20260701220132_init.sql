-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "email" character varying NOT NULL,
  "password_hash" character varying NOT NULL,
  "first_name" character varying NOT NULL,
  "last_name" character varying NOT NULL,
  "phone" character varying NULL,
  "address_street1" character varying NULL,
  "address_street2" character varying NULL,
  "address_city" character varying NULL,
  "address_state" character varying NULL,
  "address_zip" character varying NULL,
  PRIMARY KEY ("id")
);
-- Create index "users_email_key" to table: "users"
CREATE UNIQUE INDEX "users_email_key" ON "users" ("email");

-- Create "prescriptions" table
CREATE TABLE "prescriptions" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "user_id" uuid NOT NULL,
  "document_key" character varying NOT NULL,
  "physician_name" character varying NOT NULL,
  "med_name" character varying NOT NULL,
  "med_strength_value" character varying NOT NULL,
  "med_strength_unit" character varying NOT NULL,
  "quantity" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "prescription_user_id" to table: "prescriptions"
CREATE INDEX "prescription_user_id" ON "prescriptions" ("user_id");

-- Create index "order_active_offer_uniq" to table: "orders"
CREATE UNIQUE INDEX "order_active_offer_uniq" ON "orders" ("offer_id") WHERE ((status)::text <> ALL ((ARRAY['failed'::character varying, 'canceled'::character varying])::text[]));

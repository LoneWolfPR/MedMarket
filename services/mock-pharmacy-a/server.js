import express from "express";
import { readFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";

const PORT = process.env.PORT || 8080;
const API_KEY = process.env.API_KEY || "pharmacy-a-dev-key";
const ERROR_RATE = 0.05; // ~5% of requests fail, to exercise adapter error handling.

// Load the fixed catalog once at boot. Prices are stored as ranges; each
// search randomizes within [minCents, maxCents] to simulate live quoting.
const catalog = JSON.parse(
  await readFile(new URL("./catalog.json", import.meta.url), "utf8"),
);

const randInt = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min;
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const app = express();
app.use(express.json());

// Pharmacy A's auth scheme: a shared secret on the X-Api-Key header.
app.use("/api", (req, res, next) => {
  if (req.get("X-Api-Key") !== API_KEY) {
    return res.status(401).json({ error: "unauthorized" });
  }
  next();
});

// Search returns quotes with money as integer cents.
app.post("/api/v1/search", async (req, res) => {
  await delay(randInt(200, 500)); // simulated upstream latency
  if (Math.random() < ERROR_RATE) {
    return res.status(500).json({ error: "internal_error" });
  }
  const { drug = "", strength = "" } = req.body ?? {};
  const results = catalog
    .filter(
      (m) =>
        m.drug.toLowerCase() === drug.toLowerCase() &&
        (strength === "" || m.strength.toLowerCase() === strength.toLowerCase()),
    )
    .map((m) => ({
      sku: m.sku,
      drug: m.drug,
      strength: m.strength,
      priceCents: randInt(m.minCents, m.maxCents),
      currency: "USD",
      inStock: true,
      shipsInDays: m.shipsInDays,
    }));
  res.json({ results });
});

// Order confirms against a SKU and returns a tracking id the backend can
// later register with the shipping service.
app.post("/api/v1/order", async (req, res) => {
  await delay(randInt(200, 500));
  if (Math.random() < ERROR_RATE) {
    return res.status(500).json({ error: "internal_error" });
  }
  const { sku, quantity = 1 } = req.body ?? {};
  const item = catalog.find((m) => m.sku === sku);
  if (!item) {
    return res.status(404).json({ error: "unknown_sku" });
  }
  const unitCents = randInt(item.minCents, item.maxCents);
  res.status(201).json({
    orderId: `ord_${randomUUID().slice(0, 8)}`,
    trackingId: `RXD${randomUUID().replace(/-/g, "").slice(0, 12).toUpperCase()}`,
    status: "CONFIRMED",
    totalCents: unitCents * quantity,
  });
});

app.get("/healthz", (_req, res) => res.json({ status: "ok" }));

app.listen(PORT, () => console.log(`mock-pharmacy-a listening on :${PORT}`));

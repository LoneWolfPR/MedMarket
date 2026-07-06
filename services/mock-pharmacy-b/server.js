import express from "express";
import { readFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { buildSchema, graphql } from "graphql";

const PORT = process.env.PORT || 8080;
const API_TOKEN = process.env.API_TOKEN || "pharmacy-b-dev-token";
const ERROR_RATE = 0.1; // ~10% of requests fail — higher than pharmacy A.

const catalog = JSON.parse(
  await readFile(new URL("./catalog.json", import.meta.url), "utf8"),
);

// Pharmacy B speaks GraphQL and quotes money as decimal strings — a
// deliberately different contract from pharmacy A's REST + integer cents.
const schema = buildSchema(`
  type Money { amount: String!, currency: String! }
  type Stock { available: Boolean!, leadTimeDays: Int! }
  type Medication {
    code: ID!
    label: String!
    strength: String!
    unitPrice: Money!
    stock: Stock!
  }
  type OrderReceipt {
    reference: ID!
    tracking: String!
    state: String!
    grandTotal: Money!
  }
  input RecipientInput {
    name: String!
    line1: String!
    city: String!
    state: String!
    postalCode: String!
  }
  input OrderInput { code: ID!, count: Int!, recipient: RecipientInput! }
  type Query { medications(name: String!, strength: String): [Medication!]! }
  type Mutation { placeOrder(input: OrderInput!): OrderReceipt! }
`);

const randInt = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min;
// Quote a decimal-dollar string within the catalog's price range.
const quote = (min, max) => (randInt(min * 100, max * 100) / 100).toFixed(2);
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const root = {
  medications: ({ name, strength }) =>
    catalog
      .filter(
        (m) =>
          m.label.toLowerCase() === name.toLowerCase() &&
          (!strength || m.strength.toLowerCase() === strength.toLowerCase()),
      )
      .map((m) => ({
        code: m.code,
        label: m.label,
        strength: m.strength,
        unitPrice: { amount: quote(m.minPrice, m.maxPrice), currency: "USD" },
        stock: { available: true, leadTimeDays: m.leadTimeDays },
      })),
  placeOrder: ({ input }) => {
    const item = catalog.find((m) => m.code === input.code);
    if (!item) throw new Error("unknown medication code");
    const total = (Number(quote(item.minPrice, item.maxPrice)) * input.count).toFixed(2);
    return {
      reference: `mc_${randomUUID().slice(0, 8)}`,
      tracking: `MDC${randomUUID().replace(/-/g, "").slice(0, 10).toUpperCase()}`,
      state: "ACCEPTED",
      grandTotal: { amount: total, currency: "USD" },
    };
  },
};

const app = express();
app.use(express.json());

// Pharmacy B's auth scheme: a bearer token — different from A's API key.
app.use("/graphql", (req, res, next) => {
  if (req.get("Authorization") !== `Bearer ${API_TOKEN}`) {
    return res.status(401).json({ errors: [{ message: "unauthorized" }] });
  }
  next();
});

app.post("/graphql", async (req, res) => {
  await delay(randInt(100, 300)); // lower-latency profile than pharmacy A
  // B signals failure the GraphQL way — an `errors` array, not an HTTP 500.
  if (Math.random() < ERROR_RATE) {
    return res.json({ errors: [{ message: "upstream temporarily unavailable" }] });
  }
  const { query, variables } = req.body ?? {};
  const result = await graphql({
    schema,
    source: query ?? "",
    rootValue: root,
    variableValues: variables,
  });
  res.json(result);
});

app.get("/healthz", (_req, res) => res.json({ status: "ok" }));

app.listen(PORT, () => console.log(`mock-pharmacy-b listening on :${PORT}`));

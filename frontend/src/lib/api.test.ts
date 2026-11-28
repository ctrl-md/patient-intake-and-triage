// Tests for the API client. Run with: npm test
// A small fake server stands in for the Go API.

import assert from "node:assert/strict";
import { createServer, type IncomingMessage, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { after, afterEach, before, describe, it } from "node:test";

interface Seen {
  method?: string;
  url?: string;
  auth?: string;
  contentType?: string;
  body: string;
}

let server: Server;
let seen: Seen;
let reply: { status: number; body: string } = { status: 200, body: "{}" };

// api.ts reads the URL when it loads, so set it before importing.
let apiModule: typeof import("./api.ts");

before(async () => {
  server = createServer((req: IncomingMessage, res) => {
    let body = "";
    req.on("data", (chunk) => (body += chunk));
    req.on("end", () => {
      seen = {
        method: req.method,
        url: req.url,
        auth: req.headers.authorization,
        contentType: req.headers["content-type"],
        body,
      };
      res.writeHead(reply.status, { "Content-Type": "application/json" });
      res.end(reply.body);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as AddressInfo;
  process.env.NEXT_PUBLIC_API_URL = `http://127.0.0.1:${port}/`; // trailing slash on purpose
  apiModule = await import("./api.ts");
});

after(() => {
  if (server.listening) server.close();
});

afterEach(() => {
  reply = { status: 200, body: "{}" };
  apiModule.setUnauthorizedHandler(null);
});

describe("api client", () => {
  it("login posts JSON and returns the response", async () => {
    reply = { status: 200, body: JSON.stringify({ token: "t1", staff: { full_name: "Ada" } }) };

    const res = await apiModule.api.login("ada@clinic.com", "supersecret1");

    assert.equal(res.token, "t1");
    assert.equal(seen.method, "POST");
    assert.equal(seen.url, "/auth/login"); // no double slash from the trailing slash
    assert.equal(seen.contentType, "application/json");
    assert.deepEqual(JSON.parse(seen.body), { email: "ada@clinic.com", password: "supersecret1" });
    assert.equal(seen.auth, undefined);
  });

  it("sends the Bearer token", async () => {
    reply = { status: 200, body: JSON.stringify({ staff: { id: "1" } }) };
    await apiModule.api.me("my-token");
    assert.equal(seen.auth, "Bearer my-token");
    assert.equal(seen.method, "GET");
  });

  it("turns the API error message into an ApiError", async () => {
    reply = { status: 401, body: JSON.stringify({ error: "invalid email or password" }) };

    await assert.rejects(apiModule.api.login("a@b.com", "wrong"), (err: unknown) => {
      assert.ok(err instanceof apiModule.ApiError);
      assert.equal(err.status, 401);
      assert.equal(err.message, "invalid email or password");
      return true;
    });
  });

  it("does not break when an error body is not JSON", async () => {
    reply = { status: 502, body: "<html>bad gateway</html>" };

    await assert.rejects(apiModule.api.login("a@b.com", "x"), (err: unknown) => {
      assert.ok(err instanceof apiModule.ApiError);
      assert.equal(err.status, 502);
      assert.equal(err.message, "Request failed (502)");
      return true;
    });
  });

  it("calls the unauthorized handler on 401 only when a token was sent", async () => {
    let calls = 0;
    apiModule.setUnauthorizedHandler(() => calls++);
    reply = { status: 401, body: JSON.stringify({ error: "token expired" }) };

    await assert.rejects(apiModule.api.me("old-token"));
    assert.equal(calls, 1);

    // A failed login has no token, so it must not log anyone out.
    await assert.rejects(apiModule.api.login("a@b.com", "wrong"));
    assert.equal(calls, 1);
  });

  it("does not call the handler for other errors", async () => {
    let calls = 0;
    apiModule.setUnauthorizedHandler(() => calls++);
    reply = { status: 403, body: JSON.stringify({ error: "no access" }) };

    await assert.rejects(apiModule.api.me("t"));
    assert.equal(calls, 0);
  });

  it("builds the intake list query", async () => {
    reply = { status: 200, body: JSON.stringify({ intakes: [] }) };

    await apiModule.api.listIntakes("t", { status: "in_progress", limit: 10 });
    assert.equal(seen.url, "/intakes?status=in_progress&limit=10");

    await apiModule.api.listIntakes("t");
    assert.equal(seen.url, "/intakes");
  });

  it("createIntake sends the patient only when given", async () => {
    reply = { status: 201, body: JSON.stringify({ token: "x", link: "y", intake: {} }) };

    await apiModule.api.createIntake("t");
    assert.equal(seen.body, ""); // no body at all

    await apiModule.api.createIntake("t", { full_name: "Emeka Eze" });
    assert.deepEqual(JSON.parse(seen.body), { patient: { full_name: "Emeka Eze" } });
  });

  it("patient chat calls use no login and the link token", async () => {
    reply = { status: 201, body: JSON.stringify({ message: { role: "patient", content: "hi" }, emergency: false }) };
    const res = await apiModule.api.sendMessage("abc", "hi");
    assert.equal(seen.url, "/public/intakes/abc/messages");
    assert.equal(seen.auth, undefined);
    assert.deepEqual(JSON.parse(seen.body), { content: "hi" });
    assert.equal(res.emergency, false);

    reply = { status: 200, body: JSON.stringify({ status: "completed" }) };
    await apiModule.api.completeIntake("abc");
    assert.equal(seen.url, "/public/intakes/abc/complete");
    assert.equal(seen.method, "POST");
  });

  it("getTriage returns null on 404 but throws on other errors", async () => {
    reply = { status: 404, body: JSON.stringify({ error: "triage result not found" }) };
    assert.equal(await apiModule.api.getTriage("t", "id1"), null);
    assert.equal(seen.url, "/intakes/id1/triage");

    reply = { status: 500, body: JSON.stringify({ error: "something went wrong" }) };
    await assert.rejects(apiModule.api.getTriage("t", "id1"));

    reply = { status: 200, body: JSON.stringify({ triage: { final_urgency: "urgent" } }) };
    const t = await apiModule.api.getTriage("t", "id1");
    assert.equal(t?.final_urgency, "urgent");
  });

  it("runTriage posts and unwraps the result", async () => {
    reply = { status: 200, body: JSON.stringify({ triage: { final_urgency: "emergency" } }) };
    const t = await apiModule.api.runTriage("t", "id1");
    assert.equal(seen.method, "POST");
    assert.equal(seen.url, "/intakes/id1/triage/run");
    assert.equal(t.final_urgency, "emergency");
  });

  // Keep this one last: it shuts the fake server down.
  it("reports an unreachable server as status 0", async () => {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));

    await assert.rejects(apiModule.api.login("a@b.com", "x"), (err: unknown) => {
      assert.ok(err instanceof apiModule.ApiError);
      assert.equal(err.status, 0);
      assert.match(err.message, /Cannot reach the server/);
      return true;
    });
  });
});

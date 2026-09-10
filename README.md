# Chaos Draw — 1v1

A real-time two-player drawing duel. Both players join the same room code,
the server hands them a shared random seed and a synced start time, and
each browser locally (but identically) computes the same schedule of
calm/chaos windows for 60 seconds. Three ~7–8 second "DRAW NOW" windows are
guaranteed; the rest is unpredictable cursor chaos. At 60 seconds the canvas
freezes, both drawings are sent to the server, and both players see the
reveal side by side. What to actually draw is up to you to announce out
loud / in a call — the app doesn't prescribe a prompt.

## Run locally

Requires Go 1.21+ and an internet connection (to fetch the one dependency).

```bash
go mod tidy      # downloads github.com/gorilla/websocket
go run main.go
```

Open http://localhost:8080 in two separate browser windows (or two
devices on the same network, using your machine's local IP instead of
localhost), type the same room code in both, and click "Join Room".

## Deploy it live

This app needs a server that stays running and can hold WebSocket
connections open — a purely static host like **Netlify (or GitHub
Pages) can't run `main.go`**, so it can't be used on its own here.
It's fine for a page with no backend, but not for live multiplayer.
The server already serves the `static/` folder itself
(`http.FileServer` in `main.go`), so the simplest live setup is to
deploy the whole app — Go server + static files — as one unit on a
host that runs containers/Go processes:

### Render.com (easiest)
1. Push this folder to a GitHub repo.
2. On render.com: New → Web Service → connect the repo.
3. Environment: Docker (it will pick up the included `Dockerfile`
   automatically).
4. Deploy. Render gives you a public `https://your-app.onrender.com`
   URL — WebSockets work over it automatically (`wss://`).
5. The server reads the `PORT` env var Render injects automatically
   (falls back to 8080 for local runs), so no extra config is needed.

### Fly.io
1. Install `flyctl`, run `fly launch` inside this folder (it detects
   the Dockerfile).
2. `fly deploy`.
3. You get a `https://your-app.fly.dev` URL.

### Railway
1. New Project → Deploy from GitHub repo.
2. Railway auto-detects the Dockerfile and builds it.
3. Generate a public domain from the service settings.

Any of these three work well on their free tiers for a small game like
this and give you one **permanent public URL** anyone can open —
players just visit it and type a room code, no local hosting needed.

Rooms are already keyed by room code on the server, so **many rooms
run concurrently** — e.g. two friends in room `ABCD` and two other
people in room `WXYZ` play fully separate, simultaneous matches on
the same deployment.

## Notes / things you may want to tweak

- A crosshair follows your locked cursor on its own transparent layer,
  so it's always visible without ever getting painted onto the canvas.
- Pick from 5 ink colors (black/red/blue/green/yellow) in the toolbar
  above the canvas, plus an eraser toggle (draws in white, wider brush).
- Press `C` during a match to clear your own canvas entirely.
- Pointer lock (needed for the chaos-cursor effect) requires a user
  gesture, so the game clicks into it automatically once the match
  starts, but if the browser drops it, click once on the canvas.
- Currently in-memory only — rooms and images live in server RAM and
  vanish on restart. Fine for casual play; swap in a database if you
  want persistence or a match history.
- No reconnect/resume logic — if someone refreshes mid-match, they
  drop out and the other player gets an "opponent disconnected" alert.

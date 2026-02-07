# Wishlist

## Overview

Wishlist is a web app for managing your board game wishlist and collection using data from [BoardGameGeek](https://boardgamegeek.com).
It periodically syncs your wishlist and owned games from BGG, looks up prices from [Board Game Oracle](https://boardgameoracle.com) for Australian retailers, and presents an interface where you can browse your games, see where they’re cheapest, and quickly spot new additions or recent purchases.

## Features

- **BGG-backed wishlist** – Pulls your wishlist directly from your BoardGameGeek account.
- **Owned games view** – Browse the games you already own alongside your wishlist.
- **Australian price lookup** – Fetches current prices from retailers indexed by Board Game Oracle so you can see where a game is cheapest in Australia.
- **Highlights new activity** – Clearly marks new wishlist additions and recently purchased games so you can see what’s changed since you last checked.
- **Automatic background sync** – Periodically refreshes your wishlist and pricing data so the view stays up to date without manual imports.

## Project Structure

```text
wishlist/
├─ backend/
│  ├─ cmd/
│  │  └─ wishlist/
│  │     └─ main.go           # Backend entrypoint; reads env vars and starts the app
│  ├─ internal/
│  │  ├─ backend.go           # PocketBase setup, routes, background jobs
│  │  ├─ boardgamegeek.go     # BGG wishlist and collection sync logic
│  │  ├─ boardgameoracle.go   # Board Game Oracle price sync logic
│  │  └─ build.go             # Handles building the frontend
│  ├─ migrations/             # PocketBase schema migrations
│  ├─ pb_data/                # PocketBase SQLite databases and data
│  └─ pb_public/              # Static files served by the backend (includes built frontend)
└─ frontend/
   ├─ src/
   │  ├─ pages/
   │  │  └─ index.astro       # Main page showing wishlist/collection views
   │  ├─ components/
   │  │  └─ Wishlist.astro    # Wishlist UI component used on the main page
   │  ├─ layouts/
   │  │  └─ Layout.astro      # Shared page layout wrapper
   │  ├─ styles/
   │  │  └─ global.css        # Global styles layered on top of Tailwind
   │  └─ utils/
   │     └─ api.ts            # Frontend API helpers for talking to the backend
   ├─ public/                 # Static assets copied as-is into the final build
   ├─ astro.config.mjs        # Astro configuration
   ├─ tailwind.config.mjs     # Tailwind CSS configuration
   └─ package.json            # Frontend scripts and dependencies
```

## Setup

From the project root:

```bash
# Backend dependencies
cd backend
go mod download

# Frontend dependencies
cd ../frontend
npm install
```

Once dependencies are installed, configure the environment variables and start the backend/frontend as described in the sections below.

## Environment Variables

The backend is configured via a small set of environment variables:

### Backend Environment Variables

| Name             | Required?            | Description                                                                                                                       | Example           |
|------------------|----------------------|-----------------------------------------------------------------------------------------------------------------------------------|-------------------|
| `USERNAME`       | Yes                  | Your BoardGameGeek username. Used for knowing which wishlist to sync.                                                             | `my_bgg_username` |
| `BGG_AUTH_TOKEN` | Yes*                 | BoardGameGeek application auth token from <https://boardgamegeek.com/applications>. Preferred over username/password.             | `abc123...`       |
| `PASSWORD`       | Yes*                 | Your BoardGameGeek password. Only needed if you're not using an auth token.                                                       | `my_bgg_password` |
| `COUNTRY_CODE`   | Yes                  | ISO country code used when looking up prices via Board Game Oracle (e.g. AU).                                                     | `AU`              |
| `DIR`            | No (but recommended) | Path to the frontend source directory. Used as the starting point for building the frontend with `npm install` / `npm run build`. | `../frontend`     |
| `ENV`            | No                   | When set to `development`, enables extra development tooling via Outrig.                                                          | `development`     |

### Frontend Environment Variables

The frontend can optionally be configured via environment variables:

| Name              | Required? | Description                                                                          | Example                        |
|-------------------|-----------|--------------------------------------------------------------------------------------|--------------------------------|
| `PUBLIC_API_URL`  | No        | Backend API endpoint. Defaults to `http://127.0.0.1:8090` for local development.     | `https://api.example.com`      |
| `PUBLIC_SITE_URL` | No        | Public site URL for sitemap and canonical URLs. Defaults to `http://localhost:4321`. | `https://wishlist.example.com` |

To configure these for local development, create a `frontend/.env` file (see `frontend/.env.example` for a template).

**BGG authentication**  
You can authenticate with BoardGameGeek in one of two ways:

- **Recommended:** set `BGG_AUTH_TOKEN` to a token created at <https://boardgamegeek.com/applications>.
- **Alternative:** set `USERNAME` and `PASSWORD` if you prefer to authenticate with your BGG login.

Only one of these approaches needs to be configured. If both are set, the backend will use the auth token and ignore the password.

**Security note:**  
Keep your BGG credentials and token secret. Don’t commit them to version control. Use local environment variables or an `.env` file that is ignored by Git.

## Running in Development

In development, you typically run the backend and frontend in separate terminals.

### 1. Start the backend

From the project root:

```bash
cd backend

# Development environment
export ENV=development

# BoardGameGeek authentication (choose one approach)
# Recommended: application auth token from https://boardgamegeek.com/applications
export BGG_AUTH_TOKEN="your_bgg_token"

# Alternative: username/password login
# export USERNAME="your_bgg_username"
# export PASSWORD="your_bgg_password"

# Board Game Oracle region (e.g. AU)
export COUNTRY_CODE="AU"

# Frontend source directory (used when the backend builds the frontend)
export DIR="../frontend"

# Run the backend
go run ./cmd/wishlist
```

The backend listens on:

- `http://127.0.0.1:8090`

It:

- Exposes the wishlist API at `/api/v1/bgg-wishlist`.
- Schedules daily sync jobs to:
  - Pull your wishlist / owned games from BGG.
  - Fetch prices from Board Game Oracle for the configured `COUNTRY_CODE`.
- Triggers a frontend rebuild whenever data changes, using the Astro project under `DIR`.

### 1.1 PocketBase admin (initial setup and inspection)

With the backend running on `http://127.0.0.1:8090`, the PocketBase admin UI is available at:

- `http://127.0.0.1:8090/_/`

On first run:

1. Open the URL in your browser.
2. Follow the prompts to create an admin account (email + password).
3. Log in to the admin UI.

Once logged in you can:

- Inspect collections such as `items` to see wishlist/collection records and fields like `bgo_id`, `price`, etc.
- View logs and recent events via PocketBase’s built-in views.

This is useful for verifying that BGG syncs and Board Game Oracle pricing updates are behaving as expected.

### 2. Start the frontend dev server

In a second terminal:

```bash
cd frontend
npm run dev
```

Astro’s dev server (by default `http://localhost:4321`) will call the backend at `http://127.0.0.1:8090` via the helpers in `src/utils/api.ts`.  
Make sure the backend is running before you start interacting with the UI.

## Running in Production

In production, you typically:

1. Build and run the backend as a binary.
2. Let the backend manage building and serving the frontend.

### 1. Build the backend

From the project root:

```bash
cd backend
go build ./cmd/wishlist
```

This produces a `wishlist` (or `wishlist.exe` on Windows) binary in the `backend` directory.

### 2. Configure environment variables

Configure the backend environment variables as in development, but without `ENV=development`:

```bash
cd backend

# Optional: explicitly mark this as production
export ENV=production

# BoardGameGeek authentication
export BGG_AUTH_TOKEN="your_bgg_token"
# or:
# export USERNAME="your_bgg_username"
# export PASSWORD="your_bgg_password"

# Board Game Oracle region (e.g. AU)
export COUNTRY_CODE="AU"

# Frontend source directory (Astro project)
export DIR="../frontend"
```

#### 2.1. Configure frontend environment variables (optional)

Copy `frontend/.env.example` to `frontend/.env` and update the values within.

These variables will be used when the backend builds the frontend. If not set:
- `PUBLIC_API_URL` defaults to `http://127.0.0.1:8090`
- `PUBLIC_SITE_URL` defaults to `http://localhost:4321`

When the backend starts, it will:

- Listen on `http://127.0.0.1:8090` by default.
- Serve static files from `pb_public/` (including the built frontend).
- Register scheduled jobs to:
  - Sync wishlist / collection data from BGG.
  - Sync prices from Board Game Oracle for the configured `COUNTRY_CODE`.
  - Rebuild the frontend when data changes by running `npm install` and `npm run build` inside `DIR`.

**You do not need to run `npm run build` manually in production.**  
The backend’s `buildFrontend` helper will handle installing frontend dependencies and building the Astro app whenever it’s triggered by the scheduled jobs.

### 3. Run the backend binary

```bash
cd backend
./wishlist
```

Keep this process running using your preferred process manager (for example `systemd`, `supervisord`, or a container runtime).  
Expose `http://127.0.0.1:8090` behind your reverse proxy or load balancer as needed.

### PocketBase admin in production

The PocketBase admin UI is also available at:

- `http://127.0.0.1:8090/_/`

In a production deployment, treat this as an internal tool:

- Restrict access via your reverse proxy (authentication / IP allowlists), or
- Expose it only on a private network.

You can use the admin UI to inspect data in the `items` collection and to view logs if you need to debug issues on a live system.

## Deployment

A simple deployment setup looks like:

1. **Provision a host**
   - Linux VM or container with:
     - Go installed (to build the binary), or a prebuilt binary copied over.
     - Node.js and npm installed (used by the backend to build the frontend).
   - A persistent directory for `backend/pb_data` so your data survives restarts.

2. **Build the backend**

   ```bash
   cd backend
   go build ./cmd/wishlist
   ```

3. **Configure environment variables**
   - Set `BGG_AUTH_TOKEN` (or `USERNAME` / `PASSWORD`).
   - Set `COUNTRY_CODE` (e.g. `AU`).
   - Set `DIR` to the Astro project directory (usually `../frontend`).
   - Optionally set `ENV=production`.
   - **(Optional)** For custom domain deployments, create `frontend/.env` with:
     - `PUBLIC_API_URL` - Your backend URL
     - `PUBLIC_SITE_URL` - Your frontend URL

4. **Run the backend under a process manager**

   ```bash
   cd backend
   ./wishlist
   ```

   - Expose `http://127.0.0.1:8090` behind a reverse proxy such as Nginx, Caddy, or Traefik.
   - Mount `pb_data` on a persistent volume or host directory.

The backend will:

- Build the frontend using `npm install` and `npm run build` in `DIR`.
- Serve the built frontend and API from the same process.
- Keep syncing BGG data and Board Game Oracle prices according to its schedule.

## Troubleshooting

### Viewing logs

The backend logs via PocketBase’s logger.

- **In development (terminal)**:
  - Logs are printed directly to the terminal where you run:
    ```bash
    cd backend
    go run ./cmd/wishlist
    ```
  - You’ll see messages such as:
    - `Syncing BGG wishlist completed`
    - `Syncing BGO prices completed.`
    - `Country code not set, skipping BGO price sync`
    - `Failed to build frontend (npm install)` / `Failed to build frontend (npm run build)`
    - `Failed to auto-populate Board Game Oracle ID`

- **Via PocketBase admin**:
  - Open `http://127.0.0.1:8090/_/` and log in as admin.
  - Use the admin’s built-in views to inspect recent logs and events.

- **In production**:
  - If you run the binary directly, logs go to stdout/stderr:
    ```bash
    cd backend
    ./wishlist
    ```
  - Under a process manager, use its logging tools:
    - `journalctl -u wishlist.service` (systemd)
    - `docker logs <container>` (Docker)

Watching logs (either in the terminal or via the PocketBase admin) is the easiest way to understand what the sync jobs and frontend builds are doing.

### Common issues

- **Country code not set**
  - Symptom: logs show `Country code not set, skipping BGO price sync`.
  - Fix: set `COUNTRY_CODE` (e.g. `AU`) before starting the backend.

- **Missing or incorrect `bgo_id` (no price data)**
  - Symptom:
    - Board Game Oracle prices don’t appear for some games.
    - Logs may show `Failed to auto-populate Board Game Oracle ID` or `No price summary found in response`.
  - Fix:
    1. Open the PocketBase admin at `http://127.0.0.1:8090/_/` and log in.
    2. Go to the `items` collection.
    3. Find the game that’s missing prices and check its `bgo_id` field.
    4. If `bgo_id` is blank and auto-population hasn’t worked:
       - Go to the relevant game page on [Board Game Oracle](https://www.boardgameoracle.com).
       - Copy the ID from the URL (it’s the game’s key in the URL).
       - Manually paste that value into the `bgo_id` field for the item in PocketBase and save.
    5. On the next BGO price sync, the backend will use this ID when fetching prices.

- **Frontend not updating after sync**
  - Symptom: data in PocketBase looks updated, but the UI does not change.
  - Fix:
    - Check logs (terminal or admin) for messages like:
      - `Failed to build frontend (npm install)`
      - `Failed to build frontend (npm run build)`
      - `no valid frontend directory found`
    - Ensure:
      - `DIR` points at the Astro project (for example `../frontend`).
      - Node.js and npm are installed on the host so the backend can run `npm install` and `npm run build`.

- **BGG authentication issues**
  - Symptom:
    - Sync jobs fail when talking to BGG.
    - Logs often show an `EOF` error as the most common failure mode.
  - Fix:
    - If using `BGG_AUTH_TOKEN`:
      - Regenerate the token from <https://boardgamegeek.com/applications>.
      - Update the environment variable and restart the backend.
    - If using `USERNAME` / `PASSWORD`:
      - Double-check the credentials.
      - If you keep hitting `EOF`, wait a bit and retry in case of transient BGG issues.

- **Port already in use**
  - Symptom: backend fails to start with a "listen" or "address already in use" error.
  - Fix:
    - Stop any other process listening on `127.0.0.1:8090`, or
    - Adjust the PocketBase server port in your configuration and update any reverse proxy/frontend configuration that points to it.

## Tech Stack

**Backend**

- Go
- [Pocketbase](https://pocketbase.io/) for storage, routing, and scheduling background jobs
- SQLite (via PocketBase) for persistence
- [Outrig](https://outrig.run/) (development only) for local developer tooling when `ENV=development` is set
- Scheduled jobs to:
  - Sync your wishlist and owned games from BoardGameGeek
  - Sync prices from Board Game Oracle
  - Rebuild the frontend when data changes

**Frontend**

- [Astro](https://astro.build/)
- [Tailwind CSS](https://tailwindcss.com/)
- TypeScript

**Integrations**

- [BoardGameGeek](https://boardgamegeek.com) for wishlist and collection data
- [Board Game Oracle](https://boardgameoracle.com) for Australian price data


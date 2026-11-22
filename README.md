# Comic Assistant — Phase 1

A small working MVP with:

- Go backend
- React + Vite frontend
- Comic API search
- Local JSON library persistence
- Save/remove comics from the local library
- Mark comics as read/unread
- Mark comics as downloaded/not downloaded
- Optional local file path for a downloaded comic
- No download/acquisition provider is included in Phase 1

## Requirements

- Go 1.25+
- Node.js 20.19+ (or 22.12+)
- A compatible comic API URL and public/private key pair

## Run with Docker Compose

Copy `.env.example` to `.env` and set your comic API credentials, then run:

```bash
docker compose up --build
```

Open the frontend at `http://localhost:5173`. The API is available at
`http://localhost:8080`, and its library is stored in the persistent
`comic-assistant-data` Docker volume.

## 1. Configure the backend

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

Set:

```env
COMIC_ASSISTANT_API_URL=https://your-comic-api.example/v1/public/comics
COMIC_ASSISTANT_PUBLIC_KEY=your_public_key
COMIC_ASSISTANT_PRIVATE_KEY=your_private_key
PORT=8080
DATA_FILE=./data/library.json
```

The backend loads `.env` itself; no third-party dotenv dependency is required.

## 2. Run the Go API

```bash
cd backend
go run .
```

The API will listen on:

```text
http://localhost:8080
```

Health check:

```text
http://localhost:8080/api/health
```

## 3. Run the React app

In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Open the Vite URL shown by the terminal, normally:

```text
http://localhost:5173
```

## API

### Search

```http
GET /api/search?q=spider-man&limit=20
```

### Library

```http
GET /api/library
```

### Add to library

```http
POST /api/library
Content-Type: application/json
```

Body:

```json
{
  "comicId": "123",
  "title": "Example Comic",
  "series": "Amazing Spider-Man",
  "issueNumber": "1",
  "releaseDate": "2024-01-01",
  "coverUrl": "https://...",
  "description": "..."
}
```

### Update library item

```http
PATCH /api/library/:comicId
Content-Type: application/json
```

Example:

```json
{
  "read": true,
  "downloaded": true,
  "filePath": "/comics/Amazing Spider-Man #1.cbz"
}
```

### Remove

```http
DELETE /api/library/:comicId
```

## Project structure

```text
comic-assistant/
├── backend/
│   ├── main.go
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   └── styles.css
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── data/
│   └── library.json
├── .env.example
└── README.md
```

## Notes

The comic API credentials are used only by the Go backend. They are never exposed to the React browser application.

The library is intentionally stored as JSON in Phase 1 so the project has no database dependency. Phase 2 can migrate this to PostgreSQL once story arcs, relationships, reading order and larger collections are added.

import { useEffect, useState } from 'react'

const API = 'http://localhost:8080/api'

type Comic = {
  id: string
  title: string
  issueNumber: string
  description: string
  releaseDate: string
  coverUrl: string
  series: string
}

type LibraryItem = Comic & {
  comicId: string
  read: boolean
  downloaded: boolean
  filePath: string
  addedAt: string
  updatedAt: string
}

function App() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Comic[]>([])
  const [library, setLibrary] = useState<LibraryItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [view, setView] = useState<'search' | 'library'>('search')

  useEffect(() => {
    loadLibrary()
  }, [])

  async function loadLibrary() {
    try {
      const response = await fetch(`${API}/library`)
      if (!response.ok) throw new Error('Could not load library')
      setLibrary(await response.json())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load library')
    }
  }

  async function search(event: React.FormEvent) {
    event.preventDefault()
    if (!query.trim()) return

    setLoading(true)
    setError('')
    try {
      const response = await fetch(`${API}/search?q=${encodeURIComponent(query)}&limit=20`)
      const data = await response.json()
      if (!response.ok) throw new Error(data.error || 'Search failed')
      setResults(data.results)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
    } finally {
      setLoading(false)
    }
  }

  function inLibrary(id: string) {
    return library.find(item => item.comicId === id)
  }

  async function addComic(comic: Comic) {
    const response = await fetch(`${API}/library`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ...comic,
        comicId: comic.id,
      }),
    })

    const data = await response.json()
    if (!response.ok) {
      setError(data.error || 'Could not add comic')
      return
    }

    setLibrary(current => [...current, data])
  }

  async function updateComic(id: string, patch: Partial<Pick<LibraryItem, 'read' | 'downloaded' | 'filePath'>>) {
    const response = await fetch(`${API}/library/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    })

    const data = await response.json()
    if (!response.ok) {
      setError(data.error || 'Could not update comic')
      return
    }

    setLibrary(current => current.map(item => item.comicId === id ? data : item))
  }

  async function removeComic(id: string) {
    const response = await fetch(`${API}/library/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    })

    if (!response.ok) {
      const data = await response.json()
      setError(data.error || 'Could not remove comic')
      return
    }

    setLibrary(current => current.filter(item => item.comicId !== id))
  }

  return (
    <div className="app">
      <header>
        <div>
          <div className="eyebrow">PHASE 1</div>
          <h1>Comic Assistant</h1>
          <p>Search comics, build your local library, and track what you've read or downloaded.</p>
        </div>
        <nav>
          <button className={view === 'search' ? 'active' : ''} onClick={() => setView('search')}>Search</button>
          <button className={view === 'library' ? 'active' : ''} onClick={() => setView('library')}>Library ({library.length})</button>
        </nav>
      </header>

      {error && <div className="error">{error}</div>}

      {view === 'search' ? (
        <>
          <form className="search" onSubmit={search}>
            <input
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search e.g. Spider-Man"
            />
            <button type="submit" disabled={loading}>{loading ? 'Searching...' : 'Search'}</button>
          </form>

          <section>
            <div className="section-title">
              <h2>Results</h2>
              <span>{results.length} comics</span>
            </div>
            <div className="grid">
              {results.map(comic => (
                <ComicCard
                  key={comic.id}
                  comic={comic}
                  libraryItem={inLibrary(comic.id)}
                  onAdd={() => addComic(comic)}
                />
              ))}
            </div>
            {results.length === 0 && <Empty text="Search for a comic to get started." />}
          </section>
        </>
      ) : (
        <section>
          <div className="section-title">
            <h2>Your Library</h2>
            <span>{library.filter(x => x.read).length} read · {library.filter(x => x.downloaded).length} downloaded</span>
          </div>

          <div className="grid">
            {library.map(item => (
              <LibraryCard
                key={item.comicId}
                item={item}
                onUpdate={updateComic}
                onRemove={removeComic}
              />
            ))}
          </div>
          {library.length === 0 && <Empty text="Your library is empty. Search for comics and add them here." />}
        </section>
      )}
    </div>
  )
}

function ComicCard({ comic, libraryItem, onAdd }: {
  comic: Comic
  libraryItem?: LibraryItem
  onAdd: () => void
}) {
  return (
    <article className="card">
      <Cover url={comic.coverUrl} />
      <div className="card-body">
        <div className="series">{comic.series || 'Comic series unavailable'}</div>
        <h3>{comic.title}</h3>
        <div className="issue">
          {comic.issueNumber ? `Issue #${comic.issueNumber}` : 'Issue number unavailable'}
          {comic.releaseDate && ` · ${formatDate(comic.releaseDate)}`}
        </div>
        <p>{comic.description || 'No description available.'}</p>
        {libraryItem ? (
          <div className="status-row">
            <span className="pill">In library</span>
            {libraryItem.read && <span className="pill success">Read</span>}
            {libraryItem.downloaded && <span className="pill success">Downloaded</span>}
          </div>
        ) : (
          <button onClick={onAdd}>+ Add to library</button>
        )}
      </div>
    </article>
  )
}

function LibraryCard({ item, onUpdate, onRemove }: {
  item: LibraryItem
  onUpdate: (id: string, patch: Partial<Pick<LibraryItem, 'read' | 'downloaded' | 'filePath'>>) => void
  onRemove: (id: string) => void
}) {
  const [filePath, setFilePath] = useState(item.filePath)

  return (
    <article className="card">
      <Cover url={item.coverUrl} />
      <div className="card-body">
        <div className="series">{item.series || 'Comic series unavailable'}</div>
        <h3>{item.title}</h3>
        <div className="issue">
          {item.issueNumber ? `Issue #${item.issueNumber}` : 'Issue number unavailable'}
          {item.releaseDate && ` · ${formatDate(item.releaseDate)}`}
        </div>

        <div className="checks">
          <label>
            <input
              type="checkbox"
              checked={item.read}
              onChange={e => onUpdate(item.comicId, { read: e.target.checked })}
            />
            Read
          </label>
          <label>
            <input
              type="checkbox"
              checked={item.downloaded}
              onChange={e => onUpdate(item.comicId, { downloaded: e.target.checked })}
            />
            Downloaded
          </label>
        </div>

        <input
          className="filepath"
          value={filePath}
          onChange={e => setFilePath(e.target.value)}
          onBlur={() => onUpdate(item.comicId, { filePath })}
          placeholder="Local .cbz path (optional)"
        />

        <button className="danger" onClick={() => onRemove(item.comicId)}>Remove</button>
      </div>
    </article>
  )
}

function Cover({ url }: { url: string }) {
  if (!url) return <div className="cover placeholder">No cover</div>
  return <img className="cover" src={url} alt="" />
}

function Empty({ text }: { text: string }) {
  return <div className="empty">{text}</div>
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 10)
  return date.toLocaleDateString()
}

export default App

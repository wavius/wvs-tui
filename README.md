<div align="center">

<a href="https://www.themoviedb.org/"><img src="img/jinx_cat.png" alt="Cat" height="200"></a>

# wvs-tui

Watch anime, TV shows, and movies from your terminal. Uses&nbsp;&nbsp;<a href="https://www.themoviedb.org/"><img src="https://www.themoviedb.org/assets/2/v4/logos/v2/blue_long_1-8ba2ac31f354005783fab473602c34c3f4fd207150182061e425d366e4f34596.svg" height="9" alt="TMDB Logo"></a>&nbsp;&nbsp;for metadata and scrapes streaming sites via headless Chrome.

</div>

## Dependencies

- [Go](https://go.dev/) (1.21+)
- [mpv](https://mpv.io/) (for video playback)
- Google Chrome or Chromium (for headless scraping)
- Free [TMDB API Key](https://www.themoviedb.org/documentation/api)

## Quick Start

1. **Clone & Setup:**
   ```bash
   git clone https://github.com/wavius/wvs-tui.git && cd wvs-tui
   ```

2. **Install (Linux/macOS):**
   ```bash
   echo "API_KEY=your_tmdb_key_here" > .env
   make install
   ```
   *(Ensure `~/.local/bin` is in your PATH)*

   **Install (Windows):**
   ```powershell
   go build -ldflags="-X 'main/scraper.TMDBApiKey=YOUR_API_KEY'" -o wvs.exe
   ```
   *(Move the generated `wvs.exe` to a folder in your System PATH)*

## Usage

```bash
wvs <query> [flags]
```

### Commands
<table width="60%">
  <tr><th align="left" width="30%">Command</th><th align="left" width="70%">Action</th></tr>
  <tr><td><code>wvs</code></td><td>Launch interactive search mode</td></tr>
  <tr><td><code>wvs &lt;query&gt;</code></td><td>Direct search for a specific show or movie</td></tr>
</table>

### Flags
<table width="60%">
  <tr><th align="left" width="30%">Flag</th><th align="left" width="70%">Action</th></tr>
  <tr><td><code>-h</code>, <code>-help</code>, <code>--help</code></td><td>List all commands and flags</td></tr>
  <tr><td><code>-s</code></td><td>List available sources and their status</td></tr>
</table>

### Controls
<table width="60%">
  <tr><th align="left" width="30%">Key</th><th align="left" width="70%">Action</th></tr>
  <tr><td><code>Enter</code></td><td>Confirm</td></tr>
  <tr><td><code>Backspace</code></td><td>Go back to previous screen</td></tr>
  <tr><td><code>j</code> / <code>&darr;</code></td><td>Navigate down</td></tr>
  <tr><td><code>k</code> / <code>&uarr;</code></td><td>Navigate up</td></tr>
  <tr><td><code>/</code></td><td>Filter lists</td></tr>
  <tr><td><code>q</code> / <code>Esc</code> / <code>Ctrl+C</code></td><td>Quit</td></tr>
</table>

## Notes
- Posters are rendered using Halfblocks 

## TODO
- Sync watch history with Anilist/TMDB
- Video downloads
- Flags: Language (default En), Quality, Sub/Dub (default Sub), Select preferred source
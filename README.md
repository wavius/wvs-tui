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

3. **Usage:**
   ```bash
   wvs                  # Interactive mode
                        # or
   wvs Attack on Titan  # Direct search
   ```
- **Enter** to confirm
- **Backspace** to go back
- **Esc** to quit

## Notes
- Posters are rendered using Halfblocks 

## TODO
- Sync watched history with Anilist/TMDB
- Video downloads
- Flags: Language (default En), Quality, Sub/Dub (default Sub), Source
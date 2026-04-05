# VisionChat - Project Instructions

## Core Principle

**Tudo deve ser baseado na visao do agente.** O agente ve a tela, identifica elementos visuais, e interage com eles via mouse e teclado. Nunca usar atalhos que bypassem a visao (como `web_search`, `open_url`, ou navegar via URL direto). O agente DEVE olhar a tela, encontrar o que precisa, e clicar.

## Architecture

```
Frontend (TS/Vite)  →  Wails Bindings  →  Go Backend  →  llama-server (Gemma 3 4B Vision)
     60fps capture                          ├── tools/ (mouse, keyboard, filesystem)
     getUserMedia                           ├── vision/ (smart frame cache)
     getDisplayMedia                        ├── chat/ (conversation manager)
                                            └── llama/ (HTTP client, streaming, subprocess)
```

## Key Decisions

- **Modelo**: Gemma 4 E4B IT Q8_0 (via llama.cpp Vulkan). ~8GB VRAM, cabe na RTX 4070 (12GB). Superou Gemma 3 27B em benchmarks. Llama 3.2 Vision 11B NAO e suportado pelo llama-server (arquitetura `mllama` nao reconhecida).
- **llama-server**: Build b8664 do llama.cpp em `~/.cache/models/llama-cpp/`. A build do Docker (`~/.docker/bin/inference/`) nao suporta modelos de visao.
- **Video no frontend**: `getUserMedia` (webcam) e `getDisplayMedia` (screen/window) via Web APIs. Sem CGo.
- **Frame cache**: Inspirado na visao humana. Downsample para 64x64, comparacao por luminancia. 99.5% cache hit em cenas estaticas.
- **Coordenadas**: O modelo ve uma imagem de ~512x288px. As tools de click escalam automaticamente para a resolucao real da tela (ex: 2560x1440).
- **Focus durante automacao**: VisionChat se minimiza (`ShowWindow SW_MINIMIZE`) durante execucao de tools. Restaura depois.
- **Streaming**: Respostas via SSE token a token. Flag `usedStreaming` evita mensagens duplicadas.
- **TTS**: Web Speech API built-in no WebView2. Toggle VOICE no canto superior esquerdo.

## Common Issues

- **Build com `go build` falha**: Wails precisa de build tags. Sempre usar `wails build -o visionchat.exe`.
- **WebView2 zombies**: Processos `msedgewebview2.exe` podem travar o `.exe`. Fazer `taskkill //F //IM msedgewebview2.exe` antes de rebuildar.
- **Exe locked**: Se `wails build` falha com "file in use", matar WebView2 ou buildar com nome diferente (`-o visionchat2.exe`).
- **Status "Connecting..."**: O evento `server:ready` pode disparar antes do frontend registrar listeners. Ha um delay de 2s + polling fallback.
- **Ctrl+F**: O modelo tende a usar Ctrl+F para pesquisar. O prompt proibe explicitamente. Se acontecer, reforcar no prompt.
- **Mensagens duplicadas**: Streaming preenche a bubble, e o `SendMessage` retorna o texto completo. A flag `usedStreaming` previne duplicacao.

## Testing

```bash
# Unit tests (sempre rodar antes de buildar)
go test ./... -count=1

# Integration tests (requer llama-server rodando na porta 8090)
go test -tags integration -run TestIntegration -v -timeout 300s

# E2E tests simulando usuario real
go test -tags integration -run TestE2E -v -timeout 600s
```

## Build & Run

```bash
# Iniciar llama-server (se nao estiver rodando)
~/.cache/models/llama-cpp/llama-server -hf ggml-org/gemma-4-E4B-it-GGUF:Q8_0 -ngl 99 --port 8090 --flash-attn on

# Build
wails build -o visionchat.exe

# Run
./build/bin/visionchat.exe
```

## Project Structure

```
vision-chat/
  llama/          # HTTP client, streaming SSE, subprocess manager
  chat/           # Conversation history with max pairs
  vision/         # Frame processing (resize, encode) + smart cache
  tools/          # Tool registry + filesystem + screen automation (Windows API)
  app.go          # Wails bindings, tool execution loop, auto-describe
  main.go         # Entry point
  frontend/src/   # TypeScript: video capture, chat UI, TTS, keyboard shortcuts
```

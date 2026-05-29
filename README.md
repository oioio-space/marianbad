# Marienbad

Le jeu de Nim (variante misère, plateau 7-5-3) en Go + WebAssembly.

- **Modes** : contre l'ordinateur, ou 2 joueurs locaux sur le même appareil.
- **IA** : optimale (Nim-sum), avec heuristique « tricky » quand elle est en
  position perdante — elle joue toujours jusqu'au bout.
- **Hors ligne** : aucun appel réseau au runtime ; ouvre `dist/index.html`
  directement depuis le système de fichiers ou héberge le dossier `dist/`
  sur n'importe quel serveur statique.
- **Score persistant** par mode (stocké dans `localStorage`).

## Build

```bash
make            # construit dist/main.wasm + copie wasm_exec.js
make test       # exécute les tests du moteur (pur Go)
make serve      # sert dist/ sur http://localhost:8080
```

Sur Windows sans `make` :

```powershell
pwsh ./build.ps1
```

Puis ouvrir `dist/index.html`.

## Architecture

```
internal/nim/   moteur de jeu pur Go (testable sans WASM)
web/            entrypoint + controller compilé en GOOS=js GOARCH=wasm
dist/           bundle déployable (HTML/CSS/WASM/JS runtime)
```

Voir `docs/superpowers/specs/2026-05-29-marienbad-wasm-design.md` pour le
design détaillé.

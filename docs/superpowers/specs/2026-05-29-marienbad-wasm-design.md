# Marienbad — Go/WASM, single-page, offline

**Date :** 2026-05-29
**Auteur :** Mathieu Bachmann + Claude (brainstorming)
**Statut :** Design validé, prêt pour plan d'implémentation

## 1. Objectif

Construire une page web auto-suffisante (offline, `file://`-compatible) permettant à un utilisateur de jouer au jeu de Marienbad (variante misère du jeu de Nim) contre un algorithme. Toute la logique de jeu et d'IA est écrite en Go et compilée en WebAssembly. Le HTML est généré au build par `templ` + composants goilerplate. Aucune dépendance réseau au runtime.

## 1.1 Modes de jeu

Deux modes sélectionnables depuis l'écran d'accueil :

- **vs Ordinateur** : un joueur humain affronte l'IA (cf. section 3). Premier joueur tiré au sort.
- **2 joueurs locaux** : deux humains alternent sur le même appareil. Joueur 1 commence par défaut au premier match, puis on alterne le starter d'un match à l'autre.

Un bouton « Nouvelle partie » remet le plateau à zéro dans le mode courant ; un bouton « Changer de mode » revient à l'écran d'accueil.

## 1.2 Score persistant

Le score est conservé dans `localStorage` sous la clé `marianbad.score.v1`, sérialisé en JSON :

```json
{
  "vsAI":      { "humanWins": 12, "aiWins": 3 },
  "twoPlayer": { "p1Wins": 5, "p2Wins": 7 }
}
```

- Affiché en permanence dans le header (deux compteurs selon le mode actif).
- Incrémenté à la fin de chaque partie.
- Bouton « Réinitialiser le score » (avec confirmation) remet à zéro le mode courant uniquement.
- Robuste à une absence/corruption de la clé : si JSON.parse échoue, on repart de zéro silencieusement.

## 2. Règles du jeu retenues

- **Variante :** misère — *le joueur qui prend la dernière allumette perd*.
- **Plateau :** 3 rangées fixes, de haut en bas : **7, 5, 3** allumettes (15 au total).
- **Tour :** chaque joueur retire 1 à N allumettes d'**une seule** rangée par tour.
- **Premier joueur :** tiré au sort (50/50) au début de chaque partie.
- **Fin :** la partie s'arrête quand toutes les rangées sont vides ; le dernier à avoir joué perd.

## 3. Comportement de l'IA

L'IA résout le jeu de manière analytique via le **Nim-sum** (XOR des tailles de rangées), avec deux régimes :

- **Midgame (au moins une rangée ≥ 2) :**
  - Si `nimSum != 0` (position gagnante) : joue le coup qui annule le Nim-sum.
  - Si `nimSum == 0` (position perdante face à un humain parfait) : joue une heuristique **tricky** (voir 3.1).

- **Endgame misère (toutes les rangées sont 0 ou 1) :**
  Joue de manière à laisser un nombre **impair** de rangées contenant 1 allumette à l'adversaire.

### 3.1 Heuristique « tricky » en position perdante

Quand l'IA est en position perdante, aucun coup ne la sauve face à un adversaire parfait. Elle doit néanmoins **jouer jusqu'au bout**, en cherchant à maximiser les chances que l'humain commette une erreur.

Pour chaque coup légal `m`, on calcule un score qui privilégie :

1. Laisser **plusieurs rangées non triviales** (≥ 2 allumettes) — augmente la complexité du calcul du XOR pour l'humain.
2. Éviter les positions « évidentes » (`{1,1,0}`, `{n,n,0}`, etc.).
3. En cas d'égalité de score, **retirer le moins possible** — prolonge la partie et offre plus d'opportunités d'erreur.

L'algorithme précis tient en ~30 lignes ; il sera testé par propriété (cf. section 8).

## 4. Architecture

### 4.1 Arborescence

```
marianbad/
├── go.mod
├── Makefile                      # gen, css, wasm, all, serve, clean, test
├── cmd/
│   └── gen/main.go               # exécute templ, écrit dist/index.html
├── internal/
│   ├── nim/                      # moteur pur Go (zéro syscall/js)
│   │   ├── board.go
│   │   ├── board_test.go
│   │   ├── ai.go
│   │   └── ai_test.go
│   └── view/                     # composants templ + goilerplate
│       ├── layout.templ
│       └── board.templ
├── web/                          # //go:build js && wasm
│   ├── main.go                   # entrypoint WASM
│   ├── dom.go                    # helpers minces sur syscall/js
│   └── controller.go             # glue événements DOM ↔ moteur Nim
├── assets/
│   ├── tailwind.config.js
│   ├── input.css                 # @import "cards/cards.css" + utilities
│   └── cards/
│       ├── cards.css             # vendoré depuis deck-of-cards (MIT)
│       └── ATTRIBUTION.md
└── dist/                         # sortie de build, déployable telle quelle
    ├── index.html
    ├── app.css
    ├── main.wasm
    ├── wasm_exec.js
    └── ATTRIBUTION.md
```

### 4.2 Séparation des responsabilités

- **`internal/nim/`** : moteur de jeu pur, sans aucune référence à `syscall/js`. Compilable et testable sur la plateforme hôte.
- **`internal/view/`** : composants `templ` exécutés **au build uniquement**. Produisent du HTML statique.
- **`web/`** : seule partie compilée avec `GOOS=js GOARCH=wasm`. Manipule le DOM, écoute les événements, appelle le moteur.
- **`cmd/gen/`** : binaire de build qui rend les composants templ vers `dist/index.html`.

### 4.3 Build pipeline

| Cible Make | Commande | Sortie |
|---|---|---|
| `make gen` | `go run ./cmd/gen` | `dist/index.html` |
| `make css` | `tailwindcss -i assets/input.css -o dist/app.css --minify` | `dist/app.css` |
| `make wasm` | `GOOS=js GOARCH=wasm go build -o dist/main.wasm ./web` + copie `wasm_exec.js` | `dist/main.wasm`, `dist/wasm_exec.js` |
| `make` | `gen` + `css` + `wasm` | tout `dist/` |
| `make test` | `go test ./internal/...` | rapport de tests |
| `make serve` | `python -m http.server 8080 -d dist` | serveur local de QA |
| `make clean` | `rm -rf dist/` | — |

## 5. Moteur Nim (`internal/nim/`)

### 5.1 Types

```go
type Board struct {
    Rows [3]int
}

type Move struct {
    Row   int   // 0..2
    Count int   // 1..Rows[Row]
}

type Player int
const (
    P1 Player = iota   // Humain en mode vs IA, Joueur 1 en mode 2P
    P2                 // IA en mode vs IA, Joueur 2 en mode 2P
)

type Mode int
const (
    VsAI Mode = iota
    TwoPlayer
)

type GameState struct {
    Board  Board
    ToPlay Player
    Winner *Player   // nil tant que la partie n'est pas finie
}
```

### 5.2 API publique

- `NewGame(starter Player) GameState`
- `(b Board) LegalMoves() []Move`
- `(b Board) Apply(m Move) (Board, error)` — immuable, renvoie un nouveau plateau
- `(b Board) IsTerminal() bool`
- `(b Board) NimSum() int`
- `ChooseMove(b Board) Move` — décision de l'IA, encapsule midgame/endgame et l'heuristique tricky

## 6. Couche WASM / DOM (`web/`)

### 6.1 `main.go`

```go
//go:build js && wasm

func main() {
    c := controller.New()
    c.Mount()
    select {} // garde les callbacks vivants
}
```

### 6.2 `dom.go` — helpers `syscall/js`

API minimale pour rendre `controller.go` lisible : `byID`, `querySelectorAll`, `on`, `setText`, `addClass`, `removeClass`, `toggleClass`, `setAttr`.

### 6.3 `controller.go`

État maintenu :

```go
type Controller struct {
    state     nim.GameState
    selRow    int          // rangée en cours de sélection, -1 si aucune
    selFrom   int          // indice de la première carte sélectionnée (toujours sélection contiguë depuis la droite)
    handlers  []js.Func    // tous les Func à Release() au restart
    aiTimer   js.Value     // handle setTimeout, pour clearTimeout au restart
}
```

Le DOM est **rendu une fois** par templ ; le controller ne fait que `toggleClass`. Aucun re-render complet.

### 6.4 Flux d'un tour

```
startNewGame()
  ├─ Release() des handlers précédents, clearTimeout(aiTimer)
  ├─ starter := random
  ├─ state = nim.NewGame(starter)
  ├─ renderBoardClasses()  (reset .removed/.selected/.disabled sur les 15 cartes)
  ├─ updateStatus()
  └─ si ToPlay == AI → playAITurn()

[Humain joue]
  ├─ onCardClick(row, idx) :
  │     • si selRow == -1 : selRow = row, selFrom = idx
  │     • si row == selRow : selFrom = idx (re-sélection contiguë depuis la droite)
  │     • sinon : feedback shake, ignore
  │     • re-render highlights, active "Valider"
  │
  ├─ onCancel() : selRow = -1, re-render
  └─ onValidate() :
        ├─ move := {Row: selRow, Count: Rows[selRow] - selFrom}
        ├─ state.Board = state.Board.Apply(move)
        ├─ animer retrait (ajouter .removed sur les cartes concernées)
        ├─ vérifier fin → si oui : showResult() ; return
        ├─ state.ToPlay = AI
        └─ playAITurn()

playAITurn()
  ├─ ajoute .ai-thinking sur #board
  ├─ aiTimer = setTimeout(600ms, fn)
  └─ fn :
        ├─ move := nim.ChooseMove(state.Board)
        ├─ state.Board = state.Board.Apply(move)
        ├─ animer retrait
        ├─ retire .ai-thinking
        ├─ vérifier fin
        └─ state.ToPlay = Human
```

### 6.5 Règle de sélection humaine

Pour éviter toute ambiguïté, la sélection est **toujours contiguë et ancrée à la fin (droite) de la rangée** : un clic sur la carte d'indice `i` sélectionne les cartes `i..end` de cette rangée. Cliquer sur une autre carte de la même rangée déplace `selFrom`.

## 7. UI, vue, accessibilité

### 7.1 Composants templ

- `Layout()` : `<html>`, charge `app.css`, `wasm_exec.js`, instancie `Go()` et fetch `main.wasm`.
- `Board()` : structure principale (header avec bouton « Nouvelle partie », zone status, plateau de 3 rangées, boutons Annuler/Valider, footer attribution).
- `cardSlot(row, idx)` : `<button class="card back" data-row=... data-idx=... aria-label=...>`.

Les composants utilisent **goilerplate** (`templui` Pro) pour `Button` et `Card`.

### 7.2 Cartes

Cartes stylisées via **cards.css** de [deck-of-cards](https://github.com/deck-of-cards/deck-of-cards) (Daniel Imms, MIT), vendoré dans `assets/cards/`. Toutes les cartes du plateau sont **dos visible** (`<div class="card back">`). Pas d'usage de l'API JS de la lib — uniquement le CSS.

### 7.3 Classes CSS d'état (gérées par le WASM)

| Classe | Effet |
|---|---|
| `.card` | base — transition 200ms, cursor pointer |
| `.selected` | bordure dorée + translateY(-8px) |
| `.removed` | opacity 0 + scale(0) + rotate(15deg), 300ms |
| `.disabled` | grisé, pointer-events:none |
| `.ai-thinking` (sur `#board`) | cursor progress + désactive tous les clics |

### 7.4 Accessibilité

- Cartes en `<button>` natifs : focusables, navigation Tab + Espace/Entrée.
- `aria-label` descriptif par carte (« Carte rangée 1 position 3 »).
- `aria-live="polite"` sur `#status` pour annoncer les coups et résultats.
- Pas de couleur seule porteuse d'information (l'état sélectionné a aussi un offset visuel).

### 7.5 Responsive

- Desktop : cartes 80×112px.
- Mobile (`< 640px`) : cartes 48×68px ; les 3 rangées tiennent en portrait.

## 8. Tests

### 8.1 Moteur Nim (`internal/nim/`)

- `board_test.go` — table-driven :
  - `LegalMoves` exhaustif sur quelques positions clés.
  - `Apply` accepte les coups légaux, rejette les illégaux (count=0, count > Rows[row], row hors bornes).
  - `IsTerminal` correct.
  - Immutabilité (Apply ne mute pas le receveur).

- `ai_test.go` :
  - **Cas spécifiques** : `ChooseMove({7,5,3})` ramène à un Nim-sum nul ; `ChooseMove({1,1,1})` retire 1 (règle misère).
  - **Propriété 1** : pour 1000 positions aléatoires gagnantes en midgame, le coup retourné annule le Nim-sum.
  - **Propriété 2** : pour 1000 positions aléatoires, `ChooseMove` retourne toujours un coup légal.
  - **Propriété 3** : IA vs IA termine en < 16 coups, et la partie a bien un perdant identifié.
  - **Comportement** : IA(tricky) contre adversaire « random » sur 1000 parties, IA gagne ≥ 95 %. Verrouille la qualité de l'heuristique sans figer son implémentation.

### 8.2 Couche WASM

Pas de tests unitaires : couplage `syscall/js` trop fort, coût d'un mock DOM excessif pour le bénéfice. Validation par checklist QA manuelle (8.3).

### 8.3 Checklist QA manuelle (`docs/qa.md`)

1. Au chargement, le message indique qui commence.
2. Sélection contiguë depuis la droite fonctionne.
3. Clic sur autre rangée pendant une sélection : ignoré (feedback shake).
4. Annuler vide la sélection.
5. Valider applique le coup, anime le retrait, passe la main.
6. L'IA joue après ~600ms ; animation visible.
7. Fin de partie : message correct (le dernier à jouer perd).
8. « Nouvelle partie » réinitialise à tout moment, y compris pendant le tour de l'IA.
9. Recharger la page = nouvelle partie (pas de persistance).
10. `dist/index.html` ouvert en `file://` fonctionne entièrement.
11. Navigation au clavier (Tab + Espace) permet de jouer une partie complète.
12. Aucune requête réseau en DevTools une fois `dist/` chargé.

## 9. Gestion d'erreurs

| Cas | Stratégie |
|---|---|
| Coup illégal côté WASM (défense en profondeur) | `Board.Apply` renvoie une erreur, log via `console.error`, clic ignoré |
| `main.wasm` ne charge pas | HTML statique lisible (cartes affichées mais inertes) ; bandeau « WebAssembly requis » si `!window.WebAssembly` |
| Pas de `WebAssembly.instantiateStreaming` | Fallback `WebAssembly.instantiate(await resp.arrayBuffer(), ...)` |
| Reset pendant le `setTimeout` de l'IA | `clearTimeout(aiTimer)` au début de `startNewGame()` |
| Fuite des `js.Func` | Tracés dans `handlers`, `Release()` à chaque restart |
| Panic dans un callback DOM | `defer recover() + console.error` — un panic non rattrapé tuerait toute la page |

## 10. Déploiement

- **Local** : `make && open dist/index.html`.
- **Hébergement statique** : copier `dist/` sur GitHub Pages, S3, nginx — aucun runtime requis.
- **Distribuable** : `.zip` de `dist/` jouable hors ligne.
- **Taille attendue** : ~5-8 MB total (`main.wasm` ≈ 5-7 MB en `go build` standard).

## 11. Hors scope (YAGNI confirmés)

- ❌ Mode difficulté (l'algo joue toujours optimal quand il peut, tricky sinon).
- ❌ Sons.
- ❌ Internationalisation (FR uniquement).
- ❌ Animation « flip » révélant la valeur de la carte retirée (envisageable plus tard, les fichiers SVG ne sont pas vendorés pour le MVP).
- ❌ Persistance d'une partie en cours après refresh.

## 12. Crédits

- **deck-of-cards** — Daniel Imms, licence MIT — CSS des cartes.
- **goilerplate / templui Pro** — composants UI.

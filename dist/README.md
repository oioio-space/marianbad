# Marienbad — bundle prêt à jouer

## Comment jouer

Deux options selon ton besoin :

### Option 1 — Fichier unique (`standalone.html`, ~1,5 MB)

Double-clique `standalone.html` (ou ouvre-le dans un navigateur). C'est tout.
Aucune dépendance, aucun serveur, fonctionne hors ligne.

→ idéal pour : clé USB, pièce jointe email, distribution rapide.

### Option 2 — Bundle multi-fichiers (héberger sur le web)

Copie tout le contenu du dossier (`index.html`, `app.css`, `main.wasm`,
`wasm_exec.js`, `fireworks.js`, `assets/`) sur n'importe quel serveur
statique : GitHub Pages, Cloudflare Pages, nginx, S3, etc.

→ chargement plus rapide (cache HTTP, fichiers séparés mis en cache
individuellement par le navigateur).

## Compatibilité navigateur

- Chrome / Edge ≥ 80
- Firefox ≥ 113
- Safari ≥ 16.4

`standalone.html` nécessite l'API `DecompressionStream` (intégrée à ces
versions et plus récentes).

## Crédits

Voir `ATTRIBUTION.md`.

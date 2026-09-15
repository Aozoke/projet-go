# Presentation WasmRedis

Le support final contient 10 slides en format 16:9. Les extraits affiches font
entre 6 et 15 lignes et viennent directement des fichiers du projet.

Fichiers :

- `wasmredis-presentation.pptx` : version PowerPoint modifiable.
- `wasmredis-presentation.pdf` : version PDF a envoyer/ouvrir facilement.
- `notes-orales.md` : aide pour savoir quoi dire slide par slide.

Pour regenerer la presentation :

```bash
python3 presentation/build_presentation.py
```

Le script reprend des extraits exacts du code du projet, donc si une fonction change, il suffit de relancer la commande.

Pour ouvrir directement le PDF depuis WSL :

```bash
explorer.exe "$(wslpath -w presentation/wasmredis-presentation.pdf)"
```

On peut aussi double-cliquer sur `wasmredis-presentation.pdf` dans GoLand.

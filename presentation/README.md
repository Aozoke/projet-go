# Presentation WasmRedis

Le support final contient 11 slides en format 16:9. Les extraits affiches font
entre 6 et 12 lignes et viennent directement des fichiers du projet.

Fichiers :

- `wasmredis-presentation_2.pptx` : version PowerPoint prete a presenter.

Pour regenerer la presentation :

```bash
python3 presentation/build_presentation.py
```

Le script reprend des extraits exacts du code du projet et integre les notes de
presentateur directement dans le fichier PowerPoint.

Pour ouvrir directement le PowerPoint depuis WSL :

```bash
explorer.exe "$(wslpath -w 'presentation/wasmredis-presentation_2.pptx')"
```

On peut aussi double-cliquer sur `wasmredis-presentation_2.pptx` dans GoLand.

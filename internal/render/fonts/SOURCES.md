# Embedded fonts

The fonts a rendered PDF is set in, embedded in the helper and subsetted into
each PDF by the renderer. Both families are Adobe's, under the SIL Open Font
License 1.1; `OFL.txt` is the two licence files as each release ships them,
one after the other, and is embedded beside the fonts.

Only the faces the print style uses are kept: Source Sans 3 regular, bold and
italic, and Source Code Pro regular. The files are unchanged from the release
archives.

| Archive | URL | SHA-256 of the archive |
|---|---|---|
| Source Sans 3, 3.052R, TTF | https://github.com/adobe-fonts/source-sans/releases/download/3.052R/TTF-source-sans-3.052R.zip | `1b0dd1ec44b39f1dd98bbd153a1a3815f083639874ddee02c842bd601bad3d21` |
| Source Code Pro, 2.042R-u/1.062R-i, TTF | https://github.com/adobe-fonts/source-code-pro/releases/download/2.042R-u%2F1.062R-i%2F1.026R-vf/TTF-source-code-pro-2.042R-u_1.062R-i.zip | `0c85bac90d15c040b82939aa92bc8404420fccc02e37bbcb9c93a7f21abb52c6` |

| File | From the archive | SHA-256 |
|---|---|---|
| `SourceSans3-Regular.ttf` | `TTF/SourceSans3-Regular.ttf` | `4644c81b86ec9caaa76b634889968ed3c4f4f52f054855933acc7c2b21e53b0f` |
| `SourceSans3-Bold.ttf` | `TTF/SourceSans3-Bold.ttf` | `9214b9d95e4231c609802815c2646c98174e2102d0d37f88978a7f8e71006e6a` |
| `SourceSans3-It.ttf` | `TTF/SourceSans3-It.ttf` | `192afd78f0f54a3c69eaf02d43f4d9a821e9d6110e41d3d25d61a7385cd580e4` |
| `SourceCodePro-Regular.ttf` | `TTF/SourceCodePro-Regular.ttf` | `74bd80d3e42a08517cd7e1108ba3d86f2da29ac0f3065be95e0357956ab9db37` |

| Licence | URL | SHA-256 |
|---|---|---|
| Source Sans 3 | https://raw.githubusercontent.com/adobe-fonts/source-sans/3.052R/LICENSE.md | `89ad2c4f66dd29127527493e729c31e731f111cf10faf5774c3db9275ed0c22c` |
| Source Code Pro | https://raw.githubusercontent.com/adobe-fonts/source-code-pro/2.042R-u/1.062R-i/1.026R-vf/LICENSE.md | `7c940e28a5388e9bba866cf0e408edda45fe0899ba98665b8f6ab31dc5e4b8ff` |

To check a file: `shasum -a 256 internal/render/fonts/*.ttf`. The package's
test holds the same sums, so a font replaced without this table fails
`make test`.

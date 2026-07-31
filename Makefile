# kensan-lab repository tasks.
#
# Diagrams: docs/assets/platform-architecture-dark.drawio is the only diagram
# edited by hand. The light variant is derived from it through the diagram
# tokens in packages/design-tokens/tokens.json, so the palette lives in one
# place and the two files cannot drift apart.

# Pinned by digest: the PNGs are committed, so an exporter update would show up
# as a diff on a file nobody edited. Bump deliberately, and eyeball the diagram
# after — drawio-exporter 1.4.1.
DRAWIO_EXPORT ?= rlespinasse/drawio-export@sha256:76c67274d7c7cec45d7e79614e1c1af493d16f079d3db9fe9b32684f4abb67a9
ASSETS        := docs/assets
DIAGRAM       := platform-architecture
THEME         := python3 scripts/diagram-theme.py

.PHONY: help diagrams diagrams-verify

help: ## list the available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

diagrams: ## regenerate the light diagram and export both PNGs (needs docker)
	$(THEME) generate $(ASSETS)/$(DIAGRAM)-dark.drawio $(ASSETS)/$(DIAGRAM)-light.drawio
	docker run --rm -v "$(CURDIR)/$(ASSETS):/data" $(DRAWIO_EXPORT) \
		-f png -s 2 --remove-page-suffix -o . $(DIAGRAM)-dark.drawio
	docker run --rm -v "$(CURDIR)/$(ASSETS):/data" $(DRAWIO_EXPORT) \
		-f png -s 2 --remove-page-suffix -o . $(DIAGRAM)-light.drawio

diagrams-verify: ## fail if the light diagram is stale or contrast regressed
	@# The .drawio is deterministic text, so CI can regenerate and diff it. The
	@# PNGs are left out on purpose: they come out of a headless Electron and
	@# are only byte-identical on the same platform, so diffing them in CI would
	@# fail on the runner rather than on a real drift.
	$(THEME) generate $(ASSETS)/$(DIAGRAM)-dark.drawio $(ASSETS)/$(DIAGRAM)-light.drawio
	git diff --exit-code -- $(ASSETS)/$(DIAGRAM)-light.drawio

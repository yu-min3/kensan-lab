# Makefile

# envファイルの読み込み
DOTENV_FILE := .env
-include $(DOTENV_FILE)

# ===============================================
# 環境変数設定 (必ず置き換えてください)
# ===============================================
APP_REPO_NAME := goldship
IMAGE_TAG := latest
GHCR_IMAGE := ghcr.io/$(GITHUB_USER)/$(APP_REPO_NAME)/$(APP_REPO_NAME):$(IMAGE_TAG)
IMAGE_SOURCE_DIR := test-app
# -----------------------------------------------

.PHONY: login build push all clean

# ターゲット: all
# イメージのビルドとGHCRへのプッシュを一度に実行
all: build push

# ターゲット: login
# podman CLIを使ってGHCRにログイン
# 環境変数 podman_PAT が設定されていることを前提とします。
login:
	@echo "--- 🔑 GHCRにログイン中 ---"
	@if [ -z "$(GITHUB_GHCR_PAT)" ]; then \
		echo "エラー: 環境変数 GITHUB_GHCR_PAT (PATまたはFG-PAT) を設定してください。"; \
		exit 1; \
	fi
	echo "$(GITHUB_GHCR_PAT)" | podman login ghcr.io -u $(GITHUB_USER) --password-stdin
	@echo "--- ログイン成功 ---"

# ターゲット: build
# podmanイメージをビルド
build:
	@echo "--- 🔨 イメージ $(GHCR_IMAGE) をビルド中 ---"
	podman build -t $(GHCR_IMAGE) $(IMAGE_SOURCE_DIR)
	@echo "--- ビルド成功 ---"

# ターゲット: push
# ビルドしたイメージをGHCRにプッシュ
push: login
	@echo "--- ⬆️ イメージをGHCRにプッシュ中 ---"
	podman push $(GHCR_IMAGE)
	@echo "--- プッシュ成功 ---"

# ターゲット: clean
# ローカルイメージのクリーンアップ (オプション)
clean:
	@echo "--- 🗑️ ローカルイメージをクリーンアップ中 ---"
	-podman rmi $(GHCR_IMAGE)
	@echo "--- クリーンアップ完了 ---"

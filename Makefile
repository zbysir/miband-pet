# miband pet 构建入口
#   make app        构建 macOS App：server/app/build/bin/miband pet.app（包含两个手环快应用）
#   make cli        构建命令行版：server/miband
#   make quickapps  只打包手环快应用（band-app、pet-app 的 dist/*.rpk）
#   make test       跑 Go 测试

WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)

.PHONY: app cli quickapps test

quickapps:
	cd band-app && npm install --no-audit --no-fund && npx aiot build
	cd pet-app && npx aiot build

app: quickapps
	rm -f server/app/rpk/*.rpk
	cp band-app/dist/*.rpk pet-app/dist/*.rpk server/app/rpk/
	cd server/app && $(WAILS) build -clean

cli:
	cd server && go build -o miband ./cmd/miband

# server/app 依赖构建好的前端（go:embed frontend/dist），由 make app 构建，这里不测
test:
	cd server && go test ./internal/... ./cmd/...

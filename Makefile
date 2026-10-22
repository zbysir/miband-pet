# miband pet 构建入口
#   make app        构建 macOS App：server/app/build/bin/miband pet.app（包含手环上的渲染器快应用）
#   make cli        构建命令行版：server/miband
#   make quickapps  只打包手环上的渲染器快应用（render-app/dist/*.rpk）
#   make test       跑 Go 测试

WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)

.PHONY: app cli quickapps test

quickapps:
	cd render-app && npm install --no-audit --no-fund && npx aiot build

# 本机签名证书（名字见 SIGN_IDENTITY）。macOS 按“标识 + 证书”记蓝牙授权：
# 不签名的话每次构建签名都变，每次都会重新弹蓝牙授权。证书怎么建见 README。
SIGN_IDENTITY ?= miband pet local signing
APP_BUNDLE = server/app/build/bin/miband pet.app

app: quickapps
	# Mac 窗口里的猫和手环上的 Agent 猫插件用同一套帧
	rm -rf server/app/frontend/public/pet && mkdir -p server/app/frontend/public/pet
	cp -R server/plugins/agent-cat/assets/ server/app/frontend/public/pet/
	mv server/app/frontend/public/pet/fish.png server/app/frontend/public/fish.png
	# 插件预览用手环上同一份运行时
	cp render-app/src/common/runtime.js server/app/frontend/public/runtime.js
	rm -f server/app/rpk/*.rpk
	cp render-app/dist/*.rpk server/app/rpk/
	cd server/app && $(WAILS) build -clean
	@if security find-identity -p codesigning | grep -q "$(SIGN_IDENTITY)"; then \
		codesign --force --deep --identifier dev.bysir.miband-pet -s "$(SIGN_IDENTITY)" "$(APP_BUNDLE)"; \
	else \
		echo "没找到签名证书「$(SIGN_IDENTITY)」，App 未签名：每次构建后都会重新弹蓝牙授权"; \
	fi

cli:
	cd server && go build -o miband ./cmd/miband

# server/app 依赖构建好的前端（go:embed frontend/dist），由 make app 构建，这里不测
test:
	cd server && go test ./internal/... ./cmd/...

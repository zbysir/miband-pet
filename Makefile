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

# 本机签名证书（名字见 SIGN_IDENTITY）。macOS 按“标识 + 证书”记蓝牙授权：
# 不签名的话每次构建签名都变，每次都会重新弹蓝牙授权。证书怎么建见 README。
SIGN_IDENTITY ?= miband pet local signing
APP_BUNDLE = server/app/build/bin/miband pet.app

app: quickapps
	# 帧在 pet-app/src/common/pet/<哈希>/ 下（哈希防手环缓存旧图），Mac App 里不需要这层
	rm -rf server/app/frontend/public/pet && mkdir -p server/app/frontend/public/pet
	cp -R pet-app/src/common/pet/*/ server/app/frontend/public/pet/
	rm -f server/app/rpk/*.rpk
	cp band-app/dist/*.rpk pet-app/dist/*.rpk server/app/rpk/
	# Agent 猫装到手环前由 App 按选中的猫重新打包、签名，用的就是快应用的调试证书
	mkdir -p server/app/rpk/sign && cp pet-app/sign/debug/private.pem pet-app/sign/debug/certificate.pem server/app/rpk/sign/
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

.PHONY: help start android ios web build:android build:ios lint typecheck clean install

help:
	@echo "Logistics Mobile - Expo React Native App"
	@echo "=========================================="
	@echo ""
	@echo "Development:"
	@echo "  start         Start Expo development server"
	@echo "  android       Start Expo for Android"
	@echo "  ios           Start Expo for iOS"
	@echo "  web           Start Expo for web"
	@echo ""
	@echo "Build:"
	@echo "  build:android Build Android APK/AAB with EAS"
	@echo "  build:ios     Build iOS IPA with EAS"
	@echo ""
	@echo "Quality:"
	@echo "  lint          Run ESLint"
	@echo "  typecheck     Run TypeScript type checking"
	@echo ""
	@echo "Utility:"
	@echo "  clean         Clean build artifacts"
	@echo "  install       Install dependencies"

start:
	npx expo start

android:
	npx expo start --android

ios:
	npx expo start --ios

web:
	npx expo start --web

build:android:
	eas build --platform android

build:ios:
	eas build --platform ios

lint:
	npx eslint . --ext .ts,.tsx

typecheck:
	npx tsc --noEmit

clean:
	rm -rf node_modules .expo dist
	rm -f package-lock.json

install:
	npm install

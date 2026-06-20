# Tracker - 개인 일정 & 습관 트래커

🌐 **Live Demo:** https://tracker.lazzy.chat

AI 코딩 툴(Claude Code)을 활용해 개발한 1인 풀스택 프로젝트입니다.

---

## PRD (Product Requirements Document)

### 개요
개인 일정 및 습관을 추적하고 Telegram 알림을 받을 수 있는 트래커 앱입니다. 모바일(Android) 앱으로도 배포 가능합니다.

### 타겟 유저
- 일정 관리 및 습관 형성이 필요한 개인 사용자
- 텔레그램으로 알림을 받고 싶은 유저

### 주요 기능
| 기능 | 설명 |
|------|------|
| 일정 / 습관 등록 | 반복 주기 설정 (매일/특정 요일/N일 간격) |
| 히트맵 | GitHub 스타일 완료 현황 히트맵 |
| 태그 | 색상 태그로 항목 분류 |
| 알림 | Telegram 봇 알림, 지정 시간 리마인더 |
| 브리핑 | 오늘의 일정 요약 보기 |
| 간편 완료 | 링크 클릭 한 번으로 항목 완료 처리 |
| 모바일 앱 | Capacitor 기반 Android 앱 빌드 지원 |
| 푸시 알림 | FCM 기반 모바일 푸시 알림 |

### 기술 스택
- **Frontend:** React, TypeScript, Vite, Capacitor (Android)
- **Backend:** Go, Gin framework
- **알림:** Telegram Bot API, Firebase FCM
- **Auth:** JWT + Telegram 로그인
- **Deploy:** nginx reverse proxy, HTTPS

### 개발 방식
AI 코딩 툴(Claude Code)을 활용하여 요구사항 정의부터 배포까지 1인 개발

---

## 실행 방법

```bash
# 백엔드 빌드 및 실행
go build -o bin/server ./cmd/server
./bin/server

# 프론트엔드
cd tracker-web
npm install
npm run dev
```

## 환경 변수

```env
SERVER_PORT=8082
JWT_SECRET=...
TELEGRAM_BOT_TOKEN=...
FIREBASE_CREDENTIALS=...
```

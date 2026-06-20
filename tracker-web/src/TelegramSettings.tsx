import { useEffect, useState } from 'react'

function authedFetch(path: string, init: RequestInit = {}) {
  const token = localStorage.getItem('auth_token') ?? ''
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...(init.headers as Record<string, string> || {}) }
  if (token) headers['Authorization'] = `Bearer ${token}`
  return fetch(path, { ...init, headers })
}

interface Settings {
  token_set: boolean
  token_masked: string
  chat_id: string
  dnd_start: string
  dnd_end: string
  briefing_morning: string
  briefing_evening: string
  notification_mode: string
}

interface BotInfo {
  ok: boolean
  username: string
  name: string
  photo_url: string
}

const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) => ({ value: String(i), label: `${i}시` }))

export default function TelegramSettings() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [botInfo, setBotInfo] = useState<BotInfo | null>(null)
  const [token, setToken] = useState('')
  const [chatID, setChatID] = useState('')
  const [dndStart, setDndStart] = useState('')
  const [dndEnd, setDndEnd] = useState('')
  const [briefingMorning, setBriefingMorning] = useState('')
  const [briefingEvening, setBriefingEvening] = useState('')
  const [notifMode, setNotifMode] = useState('both')
  const [status, setStatus] = useState('')
  const [testing, setTesting] = useState(false)
  const [detecting, setDetecting] = useState(false)
  const [saving, setSaving] = useState(false)

  const loadSettings = () =>
    authedFetch('/api/settings/telegram')
      .then(r => r.json())
      .then((s: Settings) => {
        setSettings(s)
        setChatID(s.chat_id || '')
        setDndStart(s.dnd_start || '')
        setDndEnd(s.dnd_end || '')
        setBriefingMorning(s.briefing_morning || '')
        setBriefingEvening(s.briefing_evening || '')
        setNotifMode(s.notification_mode || 'both')
        if (s.token_set) {
          fetch('/api/settings/telegram/bot-info')
            .then(r => r.json())
            .then((b: BotInfo) => setBotInfo(b.ok ? b : null))
            .catch(() => {})
        }
      })

  useEffect(() => { loadSettings() }, [])

  const handleSave = async () => {
    setSaving(true)
    setStatus('')
    try {
      await authedFetch('/api/settings/telegram', {
        method: 'PUT',
        body: JSON.stringify({
          token: token || undefined,
          chat_id: chatID || undefined,
          dnd_start: dndStart,
          dnd_end: dndEnd,
          briefing_morning: briefingMorning,
          briefing_evening: briefingEvening,
          notification_mode: notifMode,
        }),
      })
      setToken('')
      setStatus('✅ 저장됐어요')
      await loadSettings()
    } catch {
      setStatus('❌ 저장 실패')
    } finally {
      setSaving(false)
    }
  }

  const handleDetect = async () => {
    setDetecting(true)
    setStatus('')
    try {
      const res = await authedFetch('/api/settings/telegram/detect-chat', {
        method: 'POST',
        body: JSON.stringify({ token }),
      }).then(r => r.json())
      if (res.chat_id) {
        setChatID(res.chat_id)
        setStatus(`채팅 ID 감지됨: ${res.chat_id}`)
      } else {
        setStatus(res.hint || '감지 실패')
      }
    } catch {
      setStatus('❌ 감지 실패')
    } finally {
      setDetecting(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setStatus('')
    try {
      const res = await authedFetch('/api/settings/telegram/test', {
        method: 'POST',
        body: '{}',
      }).then(r => r.json())
      if (res.ok) setStatus('✅ 테스트 메시지 전송 성공!')
      else setStatus(`❌ ${res.error}`)
    } catch {
      setStatus('❌ 전송 실패')
    } finally {
      setTesting(false)
    }
  }

  return (
    <div style={{ marginTop: 24 }}>

      {/* Bot status card */}
      {settings?.token_set && (
        <div style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: 12, padding: '12px 14px', marginBottom: 20, display: 'flex', alignItems: 'center', gap: 12 }}>
          {botInfo?.photo_url ? (
            <a href={`https://t.me/${botInfo.username}`} target="_blank" rel="noreferrer">
              <img src={botInfo.photo_url} alt="bot" style={{ width: 44, height: 44, borderRadius: '50%', objectFit: 'cover', display: 'block' }} />
            </a>
          ) : (
            <a href={botInfo ? `https://t.me/${botInfo.username}` : '#'} target="_blank" rel="noreferrer"
              style={{ width: 44, height: 44, borderRadius: '50%', background: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 22, textDecoration: 'none' }}>
              🤖
            </a>
          )}
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 13, fontWeight: 600 }}>
              {botInfo ? botInfo.name : '봇 연결됨'}
              {botInfo?.username && (
                <a href={`https://t.me/${botInfo.username}`} target="_blank" rel="noreferrer"
                  style={{ fontSize: 11, color: 'var(--accent)', marginLeft: 8, textDecoration: 'none' }}>
                  @{botInfo.username} ↗
                </a>
              )}
            </div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>
              {settings.token_masked}
              {settings.chat_id && <span style={{ marginLeft: 8 }}>채팅 ID: {settings.chat_id}</span>}
            </div>
          </div>
          <div style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--green)', flexShrink: 0 }} />
        </div>
      )}

      {/* Briefing */}
      <div style={{ marginBottom: 16 }}>
        <h3 style={{ fontSize: 13, fontWeight: 700, marginBottom: 4 }}>📋 브리핑</h3>
        <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 10 }}>
          아침엔 오늘 할 일/습관 요약, 저녁엔 달성 보고서를 보내줘요.
        </p>
        <div className="form-row">
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>🌅 아침 브리핑</label>
            <input className="form-input" type="time" value={briefingMorning} onChange={e => setBriefingMorning(e.target.value)} style={{ colorScheme: 'dark' }} />
          </div>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>🌙 저녁 브리핑</label>
            <input className="form-input" type="time" value={briefingEvening} onChange={e => setBriefingEvening(e.target.value)} style={{ colorScheme: 'dark' }} />
          </div>
        </div>
      </div>

      {/* DND */}
      <div style={{ marginBottom: 16 }}>
        <h3 style={{ fontSize: 13, fontWeight: 700, marginBottom: 4 }}>🌙 방해 금지 시간</h3>
        <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 10 }}>
          설정한 시간대에는 알림을 보내지 않아요.
          {dndStart && dndEnd && <strong style={{ color: 'var(--text)', marginLeft: 4 }}>현재: {dndStart}시 ~ {dndEnd}시</strong>}
        </p>
        <div className="form-row">
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>시작</label>
            <select className="form-select" value={dndStart} onChange={e => setDndStart(e.target.value)}>
              <option value="">없음</option>
              {HOUR_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </div>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>종료</label>
            <select className="form-select" value={dndEnd} onChange={e => setDndEnd(e.target.value)}>
              <option value="">없음</option>
              {HOUR_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Notification mode */}
      <div style={{ marginBottom: 16 }}>
        <h3 style={{ fontSize: 13, fontWeight: 700, marginBottom: 4 }}>📣 알림 수신 방식</h3>
        <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 10 }}>
          알림을 어떤 채널로 받을지 선택하세요.
        </p>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {[
            { value: 'both', label: '📱+💬 둘 다' },
            { value: 'telegram', label: '💬 텔레그램만' },
            { value: 'app', label: '📱 앱만' },
          ].map(opt => (
            <button
              key={opt.value}
              onClick={() => setNotifMode(opt.value)}
              style={{
                padding: '7px 14px',
                borderRadius: 8,
                border: `1.5px solid ${notifMode === opt.value ? 'var(--accent)' : 'var(--border)'}`,
                background: notifMode === opt.value ? 'var(--accent)' : 'var(--bg-elevated)',
                color: notifMode === opt.value ? '#fff' : 'var(--text)',
                fontSize: 13,
                cursor: 'pointer',
                fontWeight: notifMode === opt.value ? 700 : 400,
              }}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 20 }}>
        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
          {saving ? '저장 중...' : '저장'}
        </button>
        <button className="btn btn-ghost" onClick={handleTest} disabled={testing}>
          {testing ? '전송 중...' : '테스트 메시지'}
        </button>
      </div>

      {status && (
        <div style={{ marginBottom: 16, fontSize: 13, color: status.startsWith('✅') ? 'var(--green)' : status.startsWith('❌') ? 'var(--red)' : 'var(--text-muted)' }}>
          {status}
        </div>
      )}

      {/* Bot connection — collapsed at bottom */}
      <div style={{ borderTop: '1px solid var(--border)', paddingTop: 16 }}>
        <h3 style={{ fontSize: 13, fontWeight: 700, marginBottom: 10 }}>🔗 봇 연결 설정</h3>
        <div className="form-row" style={{ marginBottom: 8 }}>
          <input
            className="form-input full"
            type="password"
            placeholder="새 봇 토큰 (예: 123456789:AAH...)"
            value={token}
            onChange={e => setToken(e.target.value)}
          />
        </div>
        <div className="form-row" style={{ marginBottom: 12 }}>
          <input
            className="form-input"
            placeholder="채팅 ID"
            value={chatID}
            onChange={e => setChatID(e.target.value)}
          />
          <button className="btn btn-ghost" onClick={handleDetect} disabled={detecting} style={{ whiteSpace: 'nowrap' }}>
            {detecting ? '감지 중...' : '자동 감지'}
          </button>
        </div>
        <div style={{ fontSize: 11, color: 'var(--text-muted)', lineHeight: 1.8 }}>
          1. @BotFather → /newbot → 토큰 복사<br />
          2. 봇에 메시지 보내기 → 자동 감지 클릭<br />
          3. 저장 후 테스트 메시지 확인
        </div>
      </div>
    </div>
  )
}

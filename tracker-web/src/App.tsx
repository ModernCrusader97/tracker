import { useEffect, useRef, useState } from 'react'
import './index.css'
import Heatmap from './Heatmap'
import TelegramSettings from './TelegramSettings'
import EditModal from './EditModal'
import Briefing, { HOLIDAYS, isScheduledToday, todayStr } from './Briefing'
import { TAG_COLORS, tagColor, REMIND_OPTIONS, REMIND_BEFORE_OPTIONS, HOUR_OPTIONS } from './constants'
import { LocalNotifications } from '@capacitor/local-notifications'
import { PushNotifications } from '@capacitor/push-notifications'

const isCapacitor = !!(window as any).Capacitor

async function initNotifChannel() {
  if (!isCapacitor) return
  try {
    await LocalNotifications.createChannel({
      id: 'tracker-alerts',
      name: '트래커 알림',
      importance: 5,
      visibility: 1,
      vibration: true,
    })
  } catch { /* ignore */ }
}

async function registerFCM() {
  if (!isCapacitor) return
  try {
    let perm = await PushNotifications.checkPermissions()
    if (perm.receive === 'prompt') {
      perm = await PushNotifications.requestPermissions()
    }
    if (perm.receive !== 'granted') return
    await PushNotifications.register()
    PushNotifications.addListener('registration', async (token) => {
      try {
        const authToken = localStorage.getItem('auth_token') ?? ''
        await fetch('/api/settings/fcm-token', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${authToken}` },
          body: JSON.stringify({ token: token.value }),
        })
      } catch { /* ignore */ }
    })
  } catch { /* ignore */ }
}

async function showLocalNotification(title: string, body: string, id: number) {
  if (isCapacitor) {
    try {
      const perm = await LocalNotifications.checkPermissions()
      if (perm.display !== 'granted') {
        const req = await LocalNotifications.requestPermissions()
        if (req.display !== 'granted') return
      }
      await LocalNotifications.schedule({
        notifications: [{
          title, body, id,
          channelId: 'tracker-alerts',
          schedule: { at: new Date(Date.now() + 500) },
          smallIcon: 'ic_stat_icon_config_sample',
          iconColor: '#3b82f6',
        }]
      })
    } catch { /* ignore */ }
  } else if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
    new Notification(title, { body })
  }
}

type ItemType = 'todo' | 'habit'
type FreqType = 'daily' | 'weekly' | 'days_of_week'

interface Item {
  id: number
  type: ItemType
  title: string
  note: string
  done: boolean
  done_at?: string
  due_date?: string
  days_left?: number
  remind_every_min: number
  frequency: FreqType
  freq_days: string
  reminder_hour: number
  reminder_minute: number
  remind_before_min: number
  weekly_goal: number
  week_count: number
  streak: number
  last_checked_at?: string
  checked_today: boolean
  exclude_holidays: boolean
  tags: string
  icon: string
  created_at: string
  last_notified_at?: string
}

interface DayCheck { date: string; checked: boolean }
interface HabitStat { id: number; title: string; icon: string; streak: number; rate_30d: number; daily_checks: DayCheck[] }
interface TodoStat { date: string; count: number }
interface Stats { habits: HabitStat[]; todos_completed: TodoStat[]; total_completed: number; total_pending: number }

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('auth_token') ?? ''
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...(init.headers as Record<string,string> || {}) }
  if (token) headers['Authorization'] = `Bearer ${token}`
  const res = await fetch(path, { ...init, headers })
  if (res.status === 401) { localStorage.removeItem('auth_token'); window.location.reload(); throw new Error('unauthorized') }
  if (!res.ok) { const b = await res.json().catch(() => ({})); throw new Error((b as {error?:string}).error ?? 'error') }
  return res.json()
}

const api = {
  list: () => req<Item[]>('/api/items'),
  create: (body: object) => req<Item>('/api/items', { method: 'POST', body: JSON.stringify(body) }),
  update: (id: number, body: object) => req<Item>(`/api/items/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  delete: (id: number) => req<{deleted:boolean}>(`/api/items/${id}`, { method: 'DELETE' }),
  check: (id: number) => req<Item>(`/api/items/${id}/check`, { method: 'POST', body: '{}' }),
  stats: () => req<Stats>('/api/stats'),
}


const DAY_NAMES = ['일','월','화','수','목','금','토']

function freqLabel(freq: FreqType, freqDays: string): string {
  if (freq === 'days_of_week' && freqDays) {
    const days = freqDays.split(',').map(Number).sort()
    const weekdays = [1,2,3,4,5]
    const weekends = [0,6]
    if (JSON.stringify(days) === JSON.stringify(weekdays)) return '주중'
    if (JSON.stringify(days) === JSON.stringify(weekends)) return '주말'
    return days.map(d => DAY_NAMES[d]).join('')
  }
  return '매일'
}

type Tab = 'today' | 'todo' | 'habit' | 'stats' | 'settings'

export default function App() {
  const [token, setToken] = useState(localStorage.getItem('auth_token') ?? '')

  if (!token) {
    return <LoginPage onLogin={t => { localStorage.setItem('auth_token', t); setToken(t) }} />
  }

  return <AppMain />
}

function AppMain() {
  const [items, setItems] = useState<Item[]>([])
  const [tab, setTab] = useState<Tab>('today')
  const [showAdd, setShowAdd] = useState(false)
  const [expandedHeatmap, setExpandedHeatmap] = useState<number | null>(null)
  const [editingItem, setEditingItem] = useState<Item | null>(null)
  const [error, setError] = useState('')
  const [stats, setStats] = useState<Stats | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [pullY, setPullY] = useState(0)
  const touchStartY = useRef(0)

  const refresh = async () => {
    setRefreshing(true)
    try {
      const data = await api.list()
      setItems(data)
      if (tab === 'stats') setStats(await api.stats())
    } catch (e) { setError((e as Error).message) }
    finally { setRefreshing(false) }
  }

  const onTouchStart = (e: React.TouchEvent) => {
    touchStartY.current = e.touches[0].clientY
  }
  const onTouchMove = (e: React.TouchEvent) => {
    const dy = e.touches[0].clientY - touchStartY.current
    if (window.scrollY === 0 && dy > 0) setPullY(Math.min(dy, 80))
  }
  const onTouchEnd = () => {
    if (pullY > 50) refresh()
    setPullY(0)
  }

  const [title, setTitle] = useState('')
  const [note, setNote] = useState('')
  const [newItemTags, setNewItemTags] = useState('')
  const [customTagInput, setCustomTagInput] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [remindEvery, setRemindEvery] = useState(60)
  const [frequency, setFrequency] = useState<FreqType>('daily')
  const [freqDays, setFreqDays] = useState<number[]>([])
  const [reminderHour, setReminderHour] = useState(21)
  const [reminderMinute, setReminderMinute] = useState(0)
  const [remindBefore, setRemindBefore] = useState(0)
  const [weeklyGoal, setWeeklyGoal] = useState(0)
  const [excludeHolidays, setExcludeHolidays] = useState(false)

  useEffect(() => {
    initNotifChannel()
    registerFCM()
    api.list().then(setItems).catch(e => setError((e as Error).message))
  }, [])

  useEffect(() => {
    if (tab === 'stats') api.stats().then(setStats).catch(() => {})
  }, [tab])

  useEffect(() => {
    let sinceId = parseInt(localStorage.getItem('notif_since_id') ?? '0', 10)
    let active = true
    const poll = async () => {
      try {
        const notifs = await req<{id: number; text: string}[]>(`/api/notifications/poll?since_id=${sinceId}`)
        if (!active) return
        for (const n of notifs) {
          if (n.id > sinceId) {
            sinceId = n.id
            localStorage.setItem('notif_since_id', String(sinceId))
          }
          const body = n.text.replace(/<[^>]+>/g, '')
          showLocalNotification('트래커', body, n.id)
        }
      } catch { /* ignore */ }
      if (active) setTimeout(poll, 5000)
    }
    if (!isCapacitor && typeof Notification !== 'undefined' && Notification.permission === 'default') {
      Notification.requestPermission()
    }
    poll()
    return () => { active = false }
  }, [])

  const today = todayStr()
  const holiday = HOLIDAYS.has(today)

  const byTab = items.filter(it => it.type === (tab === 'today' || tab === 'stats' ? 'todo' : tab))
  const allTags = [...new Set(byTab.flatMap(it => it.tags ? it.tags.split(',').map(t => t.trim()).filter(Boolean) : []))]
  const filtered = tagFilter ? byTab.filter(it => it.tags && it.tags.split(',').map(t => t.trim()).includes(tagFilter)) : byTab
  const active = filtered.filter(it => !it.done)
  const done = filtered.filter(it => it.done)

  const handleAdd = async () => {
    if (!title.trim()) return
    try {
      const it = await api.create({
        type: tab,
        title: title.trim(),
        note: note.trim(),
        tags: newItemTags,
        due_date: tab === 'todo' ? dueDate : '',
        remind_every_min: tab === 'todo' ? remindEvery : 0,
        frequency: tab === 'habit' ? frequency : 'daily',
        freq_days: tab === 'habit' && frequency === 'days_of_week' ? freqDays.join(',') : '',
        reminder_hour: tab === 'habit' ? reminderHour : 21,
        reminder_minute: tab === 'habit' ? reminderMinute : 0,
        remind_before_min: tab === 'habit' ? remindBefore : 0,
        weekly_goal: tab === 'habit' ? weeklyGoal : 0,
        exclude_holidays: tab === 'habit' ? excludeHolidays : false,
      })
      setItems(prev => [it, ...prev])
      setTitle(''); setNote(''); setNewItemTags(''); setCustomTagInput(''); setDueDate(''); setExcludeHolidays(false); setShowAdd(false)
      setFrequency('daily'); setFreqDays([]); setReminderHour(21); setReminderMinute(0); setRemindBefore(0); setWeeklyGoal(0)
    } catch (e) { setError((e as Error).message) }
  }

  const toggleDone = async (it: Item) => {
    setItems(prev => prev.map(x => x.id === it.id ? { ...x, done: !it.done } : x))
    try {
      const updated = await api.update(it.id, { done: !it.done })
      setItems(prev => prev.map(x => x.id === it.id ? updated : x))
    } catch(e) {
      setItems(prev => prev.map(x => x.id === it.id ? it : x))
      setError((e as Error).message)
    }
  }

  const checkHabit = async (it: Item) => {
    if (it.checked_today) return
    setItems(prev => prev.map(x => x.id === it.id ? { ...x, checked_today: true } : x))
    try {
      const updated = await api.check(it.id)
      setItems(prev => prev.map(x => x.id === it.id ? updated : x))
    } catch (e) {
      setItems(prev => prev.map(x => x.id === it.id ? it : x))
      setError((e as Error).message)
    }
  }

  const del = async (it: Item) => {
    await api.delete(it.id)
    setItems(prev => prev.filter(x => x.id !== it.id))
  }

  const remindLabel = (min: number) => REMIND_OPTIONS.find(o => o.value === min)?.label ?? `${min}분마다`

  const nextNotifyLabel = (it: Item): string => {
    if (it.remind_every_min <= 0) return ''
    const base = it.last_notified_at ? new Date(it.last_notified_at) : new Date(it.created_at)
    const next = new Date(base.getTime() + it.remind_every_min * 60 * 1000)
    const diffMs = next.getTime() - Date.now()
    if (diffMs <= 0) return '곧 알림'
    const diffMin = Math.round(diffMs / 60000)
    if (diffMin < 60) return `${diffMin}분 후 알림`
    const h = Math.floor(diffMin / 60), m = diffMin % 60
    return m > 0 ? `${h}시간 ${m}분 후 알림` : `${h}시간 후 알림`
  }

  return (
    <div className="app" onTouchStart={onTouchStart} onTouchMove={onTouchMove} onTouchEnd={onTouchEnd}>
      {/* Pull-to-refresh indicator */}
      {pullY > 0 && (
        <div style={{ textAlign: 'center', height: pullY, display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 20, color: 'var(--text-muted)', transition: pullY === 0 ? 'height .2s' : 'none',
          overflow: 'hidden' }}>
          {pullY > 50 ? '🔄' : '↓'}
        </div>
      )}
      {refreshing && (
        <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-muted)', marginBottom: 8 }}>새로고침 중...</div>
      )}
      <h1>📋 트래커</h1>
      <p className="subtitle">할 일과 습관을 관리하고 텔레그램으로 알림을 받아요</p>

      <div className="tabs">
        <button className={`tab${tab === 'today' ? ' active' : ''}`} onClick={() => { setTab('today'); setShowAdd(false) }}>오늘</button>
        <button className={`tab${tab === 'todo' ? ' active' : ''}`} onClick={() => { setTab('todo'); setShowAdd(false) }}>할일</button>
        <button className={`tab${tab === 'habit' ? ' active' : ''}`} onClick={() => { setTab('habit'); setShowAdd(false) }}>습관</button>
        <button className={`tab${tab === 'stats' ? ' active' : ''}`} onClick={() => { setTab('stats'); setShowAdd(false) }}>📊</button>
        <button className={`tab${tab === 'settings' ? ' active' : ''}`} onClick={() => { setTab('settings'); setShowAdd(false) }} style={{ marginLeft: 'auto' }}>⚙️</button>
        {tab !== 'settings' && tab !== 'today' && tab !== 'stats' && (
          <button className="tab" onClick={() => setShowAdd(s => !s)}>
            {showAdd ? '✕' : '+'}
          </button>
        )}
      </div>

      {tab === 'today' && <Briefing items={items} onCheck={checkHabit} onToggleDone={toggleDone} />}
      {tab === 'settings' && (
        <>
          <TelegramSettings />
          <div style={{ marginTop: 24, borderTop: '1px solid var(--border)', paddingTop: 16 }}>
            <button className="btn btn-ghost" style={{ color: 'var(--red)', borderColor: 'var(--red)' }}
              onClick={() => { localStorage.removeItem('auth_token'); window.location.reload() }}>
              로그아웃
            </button>
          </div>
        </>
      )}
      {tab === 'stats' && <StatsView stats={stats} />}

      {error && <div style={{ color: 'var(--red)', fontSize: 13, marginBottom: 12 }}>{error}</div>}

      {tab !== 'settings' && tab !== 'today' && tab !== 'stats' && allTags.length > 0 && (
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
          <button onClick={() => setTagFilter('')}
            style={{ fontSize: 11, padding: '3px 10px', borderRadius: 12, border: '1px solid var(--border)', cursor: 'pointer',
              background: !tagFilter ? 'var(--accent)' : 'var(--bg-elevated)',
              color: !tagFilter ? '#fff' : 'var(--text-muted)' }}>
            전체
          </button>
          {allTags.map(tag => (
            <button key={tag} onClick={() => setTagFilter(tag === tagFilter ? '' : tag)}
              style={{ fontSize: 11, padding: '3px 10px', borderRadius: 12, border: `1px solid ${tagColor(tag)}44`, cursor: 'pointer',
                background: tag === tagFilter ? tagColor(tag) : `${tagColor(tag)}22`,
                color: tag === tagFilter ? '#fff' : tagColor(tag) }}>
              {tag}
            </button>
          ))}
        </div>
      )}

      {tab !== 'settings' && tab !== 'today' && showAdd && (
        <div className="add-form">
          <h3>{tab === 'todo' ? '✅ 할 일 추가' : '🔄 습관 추가'}</h3>
          <div className="form-row">
            <input
              className="form-input full"
              placeholder={tab === 'todo' ? '할 일 (예: 장보기)' : '습관 (예: 운동 30분)'}
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && !e.nativeEvent.isComposing && handleAdd()}
              autoFocus
            />
          </div>
          <div className="form-row">
            <input
              className="form-input full"
              placeholder="메모 (선택)"
              value={note}
              onChange={e => setNote(e.target.value)}
            />
          </div>
          <div style={{ marginBottom: 10 }}>
            <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>태그 (선택 · 자동 추천됨)</label>
            <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap', marginBottom: 6 }}>
              {Object.keys(TAG_COLORS).map(tag => {
                const active = newItemTags.split(',').map(t=>t.trim()).includes(tag)
                return (
                  <button key={tag} type="button"
                    onClick={() => {
                      const cur = newItemTags.split(',').map(t=>t.trim()).filter(Boolean)
                      setNewItemTags(active ? cur.filter(t=>t!==tag).join(',') : [...cur, tag].join(','))
                    }}
                    style={{ fontSize: 11, padding: '3px 10px', borderRadius: 12,
                      border: `1px solid ${tagColor(tag)}44`, cursor: 'pointer',
                      background: active ? tagColor(tag) : `${tagColor(tag)}22`,
                      color: active ? '#fff' : tagColor(tag) }}>
                    {tag}
                  </button>
                )
              })}
            </div>
            <div style={{ display: 'flex', gap: 6 }}>
              <input className="form-input" style={{ flex: 1 }}
                placeholder="직접 입력 (엔터로 추가)"
                value={customTagInput}
                onChange={e => setCustomTagInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter' && customTagInput.trim()) {
                    const cur = newItemTags.split(',').map(t=>t.trim()).filter(Boolean)
                    if (!cur.includes(customTagInput.trim())) setNewItemTags([...cur, customTagInput.trim()].join(','))
                    setCustomTagInput('')
                    e.preventDefault()
                  }
                }}
              />
            </div>
            {newItemTags && (
              <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 6 }}>
                {newItemTags.split(',').map(t=>t.trim()).filter(Boolean).map(tag => (
                  <span key={tag} onClick={() => setNewItemTags(newItemTags.split(',').map(t=>t.trim()).filter(t=>t&&t!==tag).join(','))}
                    style={{ fontSize: 11, padding: '2px 8px', borderRadius: 10, cursor: 'pointer',
                      background: `${tagColor(tag)}22`, color: tagColor(tag), border: `1px solid ${tagColor(tag)}44` }}>
                    {tag} ×
                  </span>
                ))}
              </div>
            )}
          </div>
          {tab === 'todo' ? (
            <div className="form-row">
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>마감일 (선택)</label>
                <input
                  type="date"
                  className="form-input"
                  value={dueDate}
                  onChange={e => setDueDate(e.target.value)}
                  style={{ colorScheme: 'dark' }}
                />
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>텔레그램 알림 주기</label>
                <select className="form-select" value={remindEvery} onChange={e => setRemindEvery(Number(e.target.value))}>
                  {REMIND_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
            </div>
          ) : (<>
            <div className="form-row">
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>반복 주기</label>
                <select className="form-select" value={frequency} onChange={e => { setFrequency(e.target.value as FreqType); setFreqDays([]) }}>
                  <option value="daily">매일</option>
                  <option value="days_of_week">요일 선택</option>
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>알림 시간</label>
                <div style={{ display: 'flex', gap: 4 }}>
                  <select className="form-select" value={reminderHour} onChange={e => setReminderHour(Number(e.target.value))} style={{ flex: 1 }}>
                    {HOUR_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                  </select>
                  <select className="form-select" value={reminderMinute} onChange={e => setReminderMinute(Number(e.target.value))} style={{ flex: 1 }}>
                    <option value={0}>00분</option>
                    <option value={15}>15분</option>
                    <option value={30}>30분</option>
                    <option value={45}>45분</option>
                  </select>
                </div>
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>사전 알림</label>
                <select className="form-select" value={remindBefore} onChange={e => setRemindBefore(Number(e.target.value))}>
                  {REMIND_BEFORE_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4, display: 'block' }}>주간 목표</label>
                <select className="form-select" value={weeklyGoal} onChange={e => setWeeklyGoal(Number(e.target.value))}>
                  <option value={0}>없음</option>
                  {[1,2,3,4,5,6,7].map(n => <option key={n} value={n}>주 {n}회</option>)}
                </select>
              </div>
            </div>
            <div style={{ marginBottom: 8 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13 }}>
                <input type="checkbox" checked={excludeHolidays} onChange={e => setExcludeHolidays(e.target.checked)}
                  style={{ width: 16, height: 16, accentColor: 'var(--accent)' }} />
                <span>🇰🇷 공휴일에 숨기기</span>
              </label>
            </div>
            {frequency === 'days_of_week' && (
              <div style={{ marginBottom: 8 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>요일 선택</label>
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {[
                    { label: '주중', days: [1,2,3,4,5] },
                    { label: '주말', days: [0,6] },
                  ].map(preset => (
                    <button key={preset.label} type="button"
                      onClick={() => setFreqDays(preset.days)}
                      style={{ fontSize: 11, padding: '3px 8px', borderRadius: 4, border: '1px solid var(--border)', cursor: 'pointer',
                        background: JSON.stringify(freqDays.slice().sort()) === JSON.stringify(preset.days.slice().sort()) ? 'var(--accent)' : 'var(--bg-elevated)',
                        color: JSON.stringify(freqDays.slice().sort()) === JSON.stringify(preset.days.slice().sort()) ? '#fff' : 'var(--text-muted)' }}>
                      {preset.label}
                    </button>
                  ))}
                  {(['일','월','화','수','목','금','토'] as const).map((name, wd) => (
                    <button key={wd} type="button"
                      onClick={() => setFreqDays(prev => prev.includes(wd) ? prev.filter(d => d !== wd) : [...prev, wd].sort())}
                      style={{ width: 30, height: 28, borderRadius: 4, border: '1px solid var(--border)', cursor: 'pointer', fontSize: 12,
                        background: freqDays.includes(wd) ? 'var(--accent)' : 'var(--bg-elevated)',
                        color: freqDays.includes(wd) ? '#fff' : 'var(--text-muted)' }}>
                      {name}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </>)}
          <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
            <button className="btn btn-primary" onClick={handleAdd}>추가</button>
            <button className="btn btn-ghost" onClick={() => setShowAdd(false)}>취소</button>
          </div>
        </div>
      )}

      {tab !== 'settings' && tab !== 'today' && active.length === 0 && done.length === 0 && (
        <div className="empty">
          {tab === 'todo' ? '할 일이 없어요 🎉  오른쪽 위 + 추가 버튼을 눌러보세요' : '습관이 없어요. + 추가 버튼으로 시작하세요!'}
        </div>
      )}

      {tab !== 'settings' && tab !== 'today' && active.map(it => (
        <div className="card" key={it.id}>
          <div className="item-header">
            {tab === 'todo' ? (
              <button className={`item-check${it.done ? ' done' : ''}`} onClick={() => toggleDone(it)}>
                {it.done && <span style={{ color: '#fff', fontSize: 11, fontWeight: 700 }}>✓</span>}
              </button>
            ) : (() => {
                const scheduled = isScheduledToday(it, holiday)
                return (
                  <button
                    className={`item-check habit-check${it.checked_today ? ' done' : ''}`}
                    onClick={() => scheduled && checkHabit(it)}
                    title={!scheduled ? '오늘 예정 없음' : it.checked_today ? '오늘 완료!' : '오늘 체크'}
                    style={!scheduled ? { opacity: 0.3, cursor: 'not-allowed' } : undefined}
                  >
                    {it.checked_today && <span style={{ color: '#fff', fontSize: 11, fontWeight: 700 }}>✓</span>}
                    {!scheduled && !it.checked_today && <span style={{ fontSize: 11 }}>—</span>}
                  </button>
                )
              })()}
            <div style={{ flex: 1 }}>
              <div className={`item-title${it.done ? ' done' : ''}`}>
                {it.icon && <span style={{ marginRight: 5 }}>{it.icon}</span>}
                {it.title}
              </div>
              {it.note && <div className="item-note">{it.note}</div>}
              {it.tags && (
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 4 }}>
                  {it.tags.split(',').map(t => t.trim()).filter(Boolean).map(tag => (
                    <span key={tag} style={{ fontSize: 10, padding: '1px 7px', borderRadius: 10,
                      background: `${tagColor(tag)}22`, color: tagColor(tag), border: `1px solid ${tagColor(tag)}44` }}>
                      {tag}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <button className="del-btn" style={{ fontSize: 13, marginRight: 2 }} onClick={() => setEditingItem(it)} title="수정">✏️</button>
            <button className="del-btn" onClick={() => del(it)}>×</button>
          </div>
          <div className="item-meta">
            {tab === 'todo' && it.days_left !== undefined && it.days_left !== null && (
              <span className="badge" style={{
                background: it.days_left < 0 ? 'rgba(224,82,82,.2)' : it.days_left === 0 ? 'rgba(224,82,82,.15)' : it.days_left <= 3 ? 'rgba(245,166,35,.15)' : 'var(--bg-elevated)',
                color: it.days_left < 0 ? 'var(--red)' : it.days_left === 0 ? 'var(--red)' : it.days_left <= 3 ? 'var(--yellow)' : 'var(--text-muted)',
              }}>
                {it.days_left < 0 ? `D+${-it.days_left} 초과` : it.days_left === 0 ? 'D-Day' : `D-${it.days_left}`}
              </span>
            )}
            {tab === 'todo' && it.remind_every_min > 0 && (
              <>
                <span className="badge blue">🔔 {remindLabel(it.remind_every_min)}</span>
                <span className="badge" style={{ color: 'var(--text-muted)' }}>⏱ {nextNotifyLabel(it)}</span>
              </>
            )}
            {tab === 'habit' && (
              <>
                <span className="badge">{freqLabel(it.frequency, it.freq_days)}</span>
                <span className="badge">{it.reminder_hour}:{String(it.reminder_minute ?? 0).padStart(2,'0')} 알림</span>
                {it.weekly_goal > 0 && <span className="badge" style={{ background: (it.week_count ?? 0) >= it.weekly_goal ? 'rgba(80,200,120,.15)' : 'var(--bg-elevated)', color: (it.week_count ?? 0) >= it.weekly_goal ? 'var(--green)' : 'var(--text-muted)' }}>주 {it.week_count ?? 0}/{it.weekly_goal}</span>}
                {it.streak > 0 && <span className="streak">🔥 {it.streak}일 연속</span>}
                {!isScheduledToday(it, holiday) && <span className="badge" style={{ color: 'var(--text-muted)' }}>오늘 없음</span>}
                {it.checked_today && <span className="badge green">오늘 완료 ✓</span>}
                <button
                  onClick={() => setExpandedHeatmap(expandedHeatmap === it.id ? null : it.id)}
                  style={{ marginLeft: 'auto', fontSize: 11, padding: '2px 8px', borderRadius: 4, background: 'var(--bg-elevated)', border: '1px solid var(--border)', cursor: 'pointer', color: 'var(--text-muted)' }}
                >
                  {expandedHeatmap === it.id ? '접기' : '기록 보기'}
                </button>
              </>
            )}
          </div>
          {tab === 'habit' && expandedHeatmap === it.id && (
            <Heatmap itemId={it.id} />
          )}
        </div>
      ))}

      {tab !== 'settings' && tab !== 'today' && done.length > 0 && (
        <>
          <div className="section-label">완료됨</div>
          {done.map(it => (
            <div className="card" key={it.id} style={{ opacity: 0.5 }}>
              <div className="item-header">
                <button className="item-check done" onClick={() => toggleDone(it)}>
                  <span style={{ color: '#fff', fontSize: 11, fontWeight: 700 }}>✓</span>
                </button>
                <div className="item-title done">{it.title}</div>
                <button className="del-btn" onClick={() => del(it)}>×</button>
              </div>
            </div>
          ))}
        </>
      )}
      {editingItem && (
        <EditModal
          item={editingItem}
          onSave={updated => {
            setItems(prev => prev.map(x => x.id === updated.id ? updated : x))
            setEditingItem(null)
          }}
          onClose={() => setEditingItem(null)}
        />
      )}
    </div>
  )
}

function BotLoginFlow({ onLogin }: { onLogin: (token: string) => void }) {
  const [deepLink, setDeepLink] = useState('')
  const [code, setCode] = useState('')
  const [waiting, setWaiting] = useState(false)
  const [error, setError] = useState('')

  const start = async () => {
    setError('')
    const data = await fetch('/api/auth/bot-login/init').then(r => r.json())
    setDeepLink(data.deep_link)
    setCode(data.code)
    window.open(data.deep_link, '_blank')
    setWaiting(true)
  }

  useEffect(() => {
    if (!waiting || !code) return
    let active = true
    const poll = async () => {
      const data = await fetch(`/api/auth/bot-login/poll?code=${code}`).then(r => r.json()).catch(() => ({}))
      if (data.ready && data.token) { onLogin(data.token); return }
      if (active) setTimeout(poll, 2000)
    }
    poll()
    return () => { active = false }
  }, [waiting, code, onLogin])

  return (
    <div style={{ marginTop: 32 }}>
      {!waiting ? (
        <button onClick={start} style={{ background: '#2AABEE', color: '#fff', border: 'none', borderRadius: 12, padding: '14px 28px', fontSize: 16, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 10, margin: '0 auto' }}>
          <span>✈️</span> 텔레그램으로 로그인
        </button>
      ) : (
        <div style={{ color: 'var(--text-muted)', fontSize: 14, lineHeight: 1.8 }}>
          텔레그램 봇에서 "시작" 버튼을 눌러주세요<br />
          <span style={{ fontSize: 12 }}>확인 중...</span>
          {deepLink && <div style={{ marginTop: 12 }}><a href={deepLink} style={{ color: 'var(--accent)', fontSize: 12 }}>링크 다시 열기</a></div>}
        </div>
      )}
      {error && <div style={{ color: 'var(--red)', marginTop: 8, fontSize: 13 }}>{error}</div>}
    </div>
  )
}

function LoginPage({ onLogin }: { onLogin: (token: string) => void }) {
  const [botUsername, setBotUsername] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/settings/telegram/bot-info')
      .then(r => r.json())
      .then(data => {
        if (data.ok && data.username) setBotUsername(data.username)
        setLoading(false)
      })
      .catch(() => { setError('봇 정보를 불러올 수 없어요'); setLoading(false) })
  }, [])

  useEffect(() => {
    if (!botUsername || isCapacitor) return
    ;(window as any).onTelegramAuth = async (user: Record<string, unknown>) => {
      try {
        const stringified = Object.fromEntries(Object.entries(user).map(([k, v]) => [k, String(v)]))
        const res = await fetch('/api/auth/telegram', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(stringified),
        })
        const data = await res.json()
        if (res.ok) {
          onLogin(data.token)
        } else {
          setError(data.error ?? '로그인 실패')
        }
      } catch { setError('로그인 중 오류가 발생했어요') }
    }
    const container = document.getElementById('tg-login-container')
    if (!container) return
    const el = document.createElement('script')
    el.src = 'https://telegram.org/js/telegram-widget.js?22'
    el.setAttribute('data-telegram-login', botUsername)
    el.setAttribute('data-size', 'large')
    el.setAttribute('data-onauth', 'onTelegramAuth(user)')
    el.setAttribute('data-request-access', 'write')
    el.async = true
    container.appendChild(el)
    return () => { container.innerHTML = '' }
  }, [botUsername, onLogin])

  return (
    <div className="app" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '80vh', textAlign: 'center' }}>
      <h1>📋 트래커</h1>
      <p className="subtitle">텔레그램으로 로그인하세요</p>
      {loading && <div style={{ color: 'var(--text-muted)', marginTop: 32 }}>로딩 중...</div>}
      {!loading && !botUsername && (
        <div style={{ color: 'var(--red)', marginTop: 32, lineHeight: 1.7 }}>
          텔레그램 봇이 설정되지 않았어요.<br />
          서버에 TELEGRAM_BOT_TOKEN 환경변수를 설정해주세요.
        </div>
      )}
      {error && <div style={{ color: 'var(--red)', marginTop: 8, fontSize: 13 }}>{error}</div>}
      {isCapacitor ? (
        botUsername && <BotLoginFlow onLogin={onLogin} />
      ) : (
        <div id="tg-login-container" style={{ marginTop: 32 }} />
      )}
    </div>
  )
}

function StatsView({ stats }: { stats: Stats | null }) {
  if (!stats) return <div className="empty">불러오는 중...</div>

  const maxTodo = Math.max(...(stats.todos_completed?.map(t => t.count) ?? [1]), 1)

  return (
    <div style={{ paddingTop: 4 }}>
      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 16, justifyContent: 'center', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--green)' }}>{stats.total_completed}</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>완료한 할 일</div>
          </div>
          <div style={{ width: 1, background: 'var(--border)' }} />
          <div>
            <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--yellow)' }}>{stats.total_pending}</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>진행 중</div>
          </div>
          <div style={{ width: 1, background: 'var(--border)' }} />
          <div>
            <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--accent)' }}>{stats.habits?.length ?? 0}</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>습관 수</div>
          </div>
        </div>
      </div>

      {stats.todos_completed && stats.todos_completed.length > 0 && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div style={{ fontWeight: 600, marginBottom: 12, fontSize: 13 }}>📅 할 일 완료 추이 (30일)</div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 3, height: 60 }}>
            {stats.todos_completed.map(t => (
              <div key={t.date} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
                <div title={`${t.date}: ${t.count}개`}
                  style={{ width: '100%', height: `${Math.round((t.count / maxTodo) * 52) + 4}px`,
                    background: 'var(--accent)', borderRadius: 2, minHeight: 4, cursor: 'default' }} />
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 4 }}>
            <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>{stats.todos_completed[0]?.date?.slice(5)}</span>
            <span style={{ fontSize: 10, color: 'var(--text-muted)' }}>{stats.todos_completed[stats.todos_completed.length - 1]?.date?.slice(5)}</span>
          </div>
        </div>
      )}

      {stats.habits && stats.habits.length > 0 && (
        <div style={{ fontWeight: 600, marginBottom: 8, fontSize: 13 }}>🔄 습관 달성률 (30일)</div>
      )}
      {stats.habits?.map(h => (
        <div className="card" key={h.id} style={{ marginBottom: 10 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
            {h.icon && <span style={{ fontSize: 18 }}>{h.icon}</span>}
            <span style={{ fontWeight: 500, flex: 1 }}>{h.title}</span>
            {h.streak > 0 && <span className="streak">🔥 {h.streak}일</span>}
            <span style={{ fontSize: 13, fontWeight: 700, color: h.rate_30d >= 0.8 ? 'var(--green)' : h.rate_30d >= 0.5 ? 'var(--yellow)' : 'var(--red)' }}>
              {Math.round(h.rate_30d * 100)}%
            </span>
          </div>
          <div style={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            {h.daily_checks?.map(dc => (
              <div key={dc.date} title={dc.date}
                style={{ width: 14, height: 14, borderRadius: 2,
                  background: dc.checked ? 'var(--green)' : 'var(--bg-elevated)',
                  border: '1px solid var(--border)', cursor: 'default' }} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

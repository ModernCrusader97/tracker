import { useState } from 'react'
import { TAG_COLORS, tagColor, REMIND_OPTIONS, REMIND_BEFORE_OPTIONS } from './constants'

interface Item {
  id: number
  type: 'todo' | 'habit'
  title: string
  note: string
  done: boolean
  done_at?: string
  due_date?: string
  days_left?: number
  remind_every_min: number
  frequency: 'daily' | 'weekly' | 'days_of_week'
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
}

interface Props {
  item: Item
  onSave: (updated: Item) => void
  onClose: () => void
}


export default function EditModal({ item, onSave, onClose }: Props) {
  const [title, setTitle] = useState(item.title)
  const [note, setNote] = useState(item.note || '')
  const [customTagInput, setCustomTagInput] = useState('')
  const [weeklyGoal, setWeeklyGoal] = useState(item.weekly_goal ?? 0)
  const [dueDate, setDueDate] = useState(item.due_date || '')
  const [remindEvery, setRemindEvery] = useState(item.remind_every_min)
  const [frequency, setFrequency] = useState(item.frequency)
  const [freqDays, setFreqDays] = useState<number[]>(
    item.freq_days ? item.freq_days.split(',').map(Number) : []
  )
  const [reminderHour, setReminderHour] = useState(item.reminder_hour)
  const [reminderMinute, setReminderMinute] = useState(item.reminder_minute ?? 0)
  const [remindBefore, setRemindBefore] = useState(item.remind_before_min)
  const [excludeHolidays, setExcludeHolidays] = useState(item.exclude_holidays ?? false)
  const [tags, setTags] = useState(item.tags || '')
  const [icon, setIcon] = useState(item.icon || '')
  const [saving, setSaving] = useState(false)

  const PRESET_ICONS = ['🏃','💊','📚','💼','🍽️','🧹','🛒','👥','🎮','💰','🧘','🚴','🎯','✍️','🎵','🛏️','💧','🌿','🏠','📱']
  const ALL_TAGS = Object.keys(TAG_COLORS)
  const activeTags = tags ? tags.split(',').map(t => t.trim()).filter(Boolean) : []
  const toggleTag = (tag: string) => {
    const cur = activeTags
    const next = cur.includes(tag) ? cur.filter(t => t !== tag) : [...cur, tag]
    setTags(next.join(','))
  }

  const [saveError, setSaveError] = useState('')

  const handleSave = async () => {
    if (!title.trim()) return
    setSaving(true)
    setSaveError('')
    try {
      const token = localStorage.getItem('auth_token') ?? ''
      const res = await fetch(`/api/items/${item.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({
          title: title.trim(),
          note: note.trim(),
          due_date: item.type === 'todo' ? dueDate : undefined,
          remind_every_min: item.type === 'todo' ? remindEvery : undefined,
          frequency: item.type === 'habit' ? frequency : undefined,
          freq_days: item.type === 'habit' && frequency === 'days_of_week' ? freqDays.join(',') : undefined,
          reminder_hour: item.type === 'habit' ? reminderHour : undefined,
          reminder_minute: item.type === 'habit' ? reminderMinute : undefined,
          remind_before_min: item.type === 'habit' ? remindBefore : undefined,
          weekly_goal: item.type === 'habit' ? weeklyGoal : undefined,
          exclude_holidays: item.type === 'habit' ? excludeHolidays : undefined,
          tags: tags.trim(),
          icon: icon.trim(),
        }),
      })
      if (res.status === 401) { localStorage.removeItem('auth_token'); window.location.reload(); return }
      if (!res.ok) { const b = await res.json().catch(() => ({})); setSaveError((b as {error?:string}).error ?? '저장 실패'); return }
      const updated = await res.json()
      onSave(updated)
    } catch (e) {
      setSaveError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}
      onClick={onClose}
    >
      <div
        style={{ background: 'var(--bg-card)', border: '1px solid var(--border)', borderRadius: 14, padding: 24, width: '100%', maxWidth: 460 }}
        onClick={e => e.stopPropagation()}
      >
        <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 16 }}>
          {item.type === 'todo' ? '할 일 수정' : '습관 수정'}
        </h3>

        <div style={{ marginBottom: 10 }}>
          <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>제목</label>
          <input
            className="form-input full"
            value={title}
            onChange={e => setTitle(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && !e.nativeEvent.isComposing && handleSave()}
            autoFocus
          />
        </div>

        <div style={{ marginBottom: 10 }}>
          <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>메모</label>
          <input
            className="form-input full"
            placeholder="메모 (선택)"
            value={note}
            onChange={e => setNote(e.target.value)}
          />
        </div>

        <div style={{ marginBottom: 10 }}>
          <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>아이콘</label>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <input
              className="form-input"
              placeholder="이모지 직접 입력"
              value={icon}
              onChange={e => setIcon(e.target.value)}
              style={{ width: 80 }}
            />
            {PRESET_ICONS.map(em => (
              <button key={em} type="button" onClick={() => setIcon(em)}
                style={{ fontSize: 18, background: icon === em ? 'var(--accent)' : 'var(--bg-elevated)',
                  border: `1px solid ${icon === em ? 'var(--accent)' : 'var(--border)'}`,
                  borderRadius: 6, cursor: 'pointer', padding: '2px 4px' }}>
                {em}
              </button>
            ))}
          </div>
        </div>

        <div style={{ marginBottom: 10 }}>
          <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>태그</label>
          <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap', marginBottom: 6 }}>
            {ALL_TAGS.map(tag => (
              <button key={tag} type="button" onClick={() => toggleTag(tag)}
                style={{ fontSize: 11, padding: '3px 10px', borderRadius: 12,
                  border: `1px solid ${tagColor(tag)}44`, cursor: 'pointer',
                  background: activeTags.includes(tag) ? tagColor(tag) : `${tagColor(tag)}22`,
                  color: activeTags.includes(tag) ? '#fff' : tagColor(tag) }}>
                {tag}
              </button>
            ))}
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            <input className="form-input" style={{ flex: 1 }}
              placeholder="직접 입력 (엔터로 추가)"
              value={customTagInput}
              onChange={e => setCustomTagInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && !e.nativeEvent.isComposing && customTagInput.trim()) {
                  if (!activeTags.includes(customTagInput.trim())) toggleTag(customTagInput.trim())
                  setCustomTagInput('')
                  e.preventDefault()
                }
              }}
            />
          </div>
          {activeTags.filter(t => !ALL_TAGS.includes(t)).length > 0 && (
            <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 6 }}>
              {activeTags.filter(t => !ALL_TAGS.includes(t)).map(tag => (
                <span key={tag} onClick={() => toggleTag(tag)}
                  style={{ fontSize: 11, padding: '2px 8px', borderRadius: 10, cursor: 'pointer',
                    background: '#6b728022', color: '#9ca3af', border: '1px solid #6b728044' }}>
                  {tag} ×
                </span>
              ))}
            </div>
          )}
        </div>

        {item.type === 'todo' && (
          <div className="form-row" style={{ marginBottom: 10 }}>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>마감일</label>
              <input
                type="date"
                className="form-input"
                value={dueDate}
                onChange={e => setDueDate(e.target.value)}
                style={{ colorScheme: 'dark' }}
              />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>알림 주기</label>
              <select className="form-select" value={remindEvery} onChange={e => setRemindEvery(Number(e.target.value))}>
                {REMIND_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </div>
          </div>
        )}

        {item.type === 'habit' && (
          <>
            <div className="form-row" style={{ marginBottom: 10 }}>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>반복 주기</label>
                <select className="form-select" value={frequency} onChange={e => { setFrequency(e.target.value as 'daily' | 'days_of_week'); setFreqDays([]) }}>
                  <option value="daily">매일</option>
                  <option value="days_of_week">요일 선택</option>
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>알림 시간</label>
                <div style={{ display: 'flex', gap: 4 }}>
                  <select className="form-select" value={reminderHour} onChange={e => setReminderHour(Number(e.target.value))} style={{ flex: 1 }}>
                    {Array.from({ length: 24 }, (_, i) => <option key={i} value={i}>{i}시</option>)}
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
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>사전 알림</label>
                <select className="form-select" value={remindBefore} onChange={e => setRemindBefore(Number(e.target.value))}>
                  {REMIND_BEFORE_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 4 }}>주간 목표</label>
                <select className="form-select" value={weeklyGoal} onChange={e => setWeeklyGoal(Number(e.target.value))}>
                  <option value={0}>없음</option>
                  {[1,2,3,4,5,6,7].map(n => <option key={n} value={n}>주 {n}회</option>)}
                </select>
              </div>
            </div>
            <div style={{ marginBottom: 10 }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13 }}>
                <input
                  type="checkbox"
                  checked={excludeHolidays}
                  onChange={e => setExcludeHolidays(e.target.checked)}
                  style={{ width: 16, height: 16, accentColor: 'var(--accent)' }}
                />
                <span>🇰🇷 공휴일에 숨기기 (주중 전용 습관)</span>
              </label>
            </div>
            {frequency === 'days_of_week' && (
              <div style={{ marginBottom: 10 }}>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>요일 선택</label>
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {[{ label: '주중', days: [1,2,3,4,5] }, { label: '주말', days: [0,6] }].map(p => (
                    <button key={p.label} type="button"
                      onClick={() => setFreqDays(p.days)}
                      style={{ fontSize: 11, padding: '3px 8px', borderRadius: 4, border: '1px solid var(--border)', cursor: 'pointer',
                        background: JSON.stringify(freqDays.slice().sort()) === JSON.stringify(p.days.slice().sort()) ? 'var(--accent)' : 'var(--bg-elevated)',
                        color: JSON.stringify(freqDays.slice().sort()) === JSON.stringify(p.days.slice().sort()) ? '#fff' : 'var(--text-muted)' }}>
                      {p.label}
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
          </>
        )}

        {saveError && <div style={{ color: '#ef4444', fontSize: 12, marginBottom: 8 }}>{saveError}</div>}
        <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
          <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
            {saving ? '저장 중...' : '저장'}
          </button>
          <button className="btn btn-ghost" onClick={onClose}>취소</button>
        </div>
      </div>
    </div>
  )
}

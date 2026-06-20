import { useMemo } from 'react'

type FreqType = 'daily' | 'weekly' | 'days_of_week'

export interface BriefingItem {
  id: number
  type: 'todo' | 'habit'
  title: string
  note: string
  done: boolean
  due_date?: string
  days_left?: number
  frequency: FreqType
  freq_days: string
  reminder_hour: number
  reminder_minute: number
  remind_every_min: number
  remind_before_min: number
  streak: number
  checked_today: boolean
  weekly_goal: number
  week_count: number
  exclude_holidays: boolean
  tags: string
  icon: string
  created_at: string
}

interface Props {
  items: BriefingItem[]
  onCheck: (item: BriefingItem) => void
  onToggleDone: (item: BriefingItem) => void
}

// Korean public holidays (YYYY-MM-DD)
export const HOLIDAYS = new Set([
  // 2025
  '2025-01-01','2025-01-28','2025-01-29','2025-01-30',
  '2025-03-01','2025-05-05','2025-05-06','2025-05-13',
  '2025-06-06','2025-08-15','2025-10-03','2025-10-05',
  '2025-10-06','2025-10-07','2025-10-09','2025-12-25',
  // 2026
  '2026-01-01','2026-02-16','2026-02-17','2026-02-18',
  '2026-03-01','2026-05-05','2026-05-24','2026-06-06',
  '2026-08-15','2026-09-29','2026-09-30','2026-10-01',
  '2026-10-03','2026-10-09','2026-12-25',
  // 2027
  '2027-01-01','2027-02-07','2027-02-08','2027-02-09',
  '2027-03-01','2027-05-05','2027-06-06','2027-08-15',
  '2027-10-03','2027-10-09','2027-10-14','2027-10-15',
  '2027-10-16','2027-12-25',
])

export function todayStr() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function isHoliday(dateStr: string) {
  return HOLIDAYS.has(dateStr)
}

export function isScheduledToday(item: { frequency: FreqType; freq_days: string; exclude_holidays: boolean }, holiday: boolean): boolean {
  const wd = new Date().getDay()
  if (item.frequency === 'days_of_week' && item.freq_days) {
    if (holiday && item.exclude_holidays) return false
    return item.freq_days.split(',').map(Number).includes(wd)
  }
  if (holiday && item.exclude_holidays) return false
  return true
}

const DAY_KOR = ['일', '월', '화', '수', '목', '금', '토']

const HOLIDAY_NAMES: Record<string, string> = {
  '2025-01-01': '신정', '2025-01-28': '설날', '2025-01-29': '설날', '2025-01-30': '설날',
  '2025-03-01': '삼일절', '2025-05-05': '어린이날', '2025-05-06': '어린이날 대체', '2025-05-13': '부처님오신날',
  '2025-06-06': '현충일', '2025-08-15': '광복절', '2025-10-03': '개천절',
  '2025-10-05': '추석', '2025-10-06': '추석', '2025-10-07': '추석', '2025-10-09': '한글날', '2025-12-25': '크리스마스',
  '2026-01-01': '신정', '2026-02-16': '설날', '2026-02-17': '설날', '2026-02-18': '설날',
  '2026-03-01': '삼일절', '2026-05-05': '어린이날', '2026-05-24': '부처님오신날',
  '2026-06-06': '현충일', '2026-08-15': '광복절',
  '2026-09-29': '추석', '2026-09-30': '추석', '2026-10-01': '추석',
  '2026-10-03': '개천절', '2026-10-09': '한글날', '2026-12-25': '크리스마스',
}

export default function Briefing({ items, onCheck, onToggleDone }: Props) {
  const today = todayStr()
  const holiday = isHoliday(today)
  const holidayName = HOLIDAY_NAMES[today]

  const dayName = DAY_KOR[new Date().getDay()]
  const month = new Date().getMonth() + 1
  const date = new Date().getDate()

  const { overdue, dueToday, dueThisWeek, todayHabits } = useMemo(() => {
    const todos = items.filter(it => it.type === 'todo' && !it.done)
    const overdue = todos.filter(it => it.days_left !== undefined && it.days_left !== null && it.days_left < 0)
    const dueToday = todos.filter(it => it.days_left === 0)
    const dueThisWeek = todos.filter(it => it.days_left !== undefined && it.days_left !== null && it.days_left > 0 && it.days_left <= 7)
    const habits = items.filter(it => it.type === 'habit' && isScheduledToday(it, holiday))
    return { overdue, dueToday, dueThisWeek, todayHabits: habits }
  }, [items, today, holiday])

  const checkedCount = todayHabits.filter(h => h.checked_today).length
  const allDone = overdue.length === 0 && dueToday.length === 0 && (todayHabits.length === 0 || checkedCount === todayHabits.length)

  return (
    <div>
      {/* Header */}
      <div style={{
        background: 'var(--bg-elevated)',
        borderRadius: 12,
        padding: '16px 20px',
        marginBottom: 16,
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <div>
          <div style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 4, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <span>{month}월 {date}일 ({dayName}요일)</span>
            {holiday && (
              <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 8, background: 'rgba(99,102,241,.2)', color: '#818cf8', fontWeight: 600 }}>
                🇰🇷 {holidayName ?? '공휴일'}
              </span>
            )}
          </div>
          <div style={{ fontSize: 18, fontWeight: 700 }}>
            {overdue.length > 0
              ? '⚠️ 초과된 일정이 있어요'
              : dueToday.length > 0
              ? '🔴 오늘 마감이 있어요'
              : allDone
              ? '🎉 오늘 모든 일정 완료!'
              : '📋 오늘 일정을 확인하세요'}
          </div>
        </div>
        <div style={{ textAlign: 'right', fontSize: 12, color: 'var(--text-muted)', lineHeight: 1.9 }}>
          <div>미완료 할 일 <b style={{ color: 'var(--text)' }}>{items.filter(it => it.type === 'todo' && !it.done).length}개</b></div>
          <div>오늘 습관 <b style={{ color: checkedCount === todayHabits.length && todayHabits.length > 0 ? 'var(--green)' : 'var(--text)' }}>{checkedCount}/{todayHabits.length}</b></div>
        </div>
      </div>

      {/* Overdue */}
      {overdue.length > 0 && (
        <Section title="🚨 마감 초과" accent="var(--red)">
          {overdue.map(it => <TodoRow key={it.id} item={it} onToggle={onToggleDone} />)}
        </Section>
      )}

      {/* Due today */}
      {dueToday.length > 0 && (
        <Section title="🔴 오늘 마감" accent="#f59e0b">
          {dueToday.map(it => <TodoRow key={it.id} item={it} onToggle={onToggleDone} />)}
        </Section>
      )}

      {/* Today habits */}
      {todayHabits.length > 0 && (
        <Section title={`🔄 오늘 습관 (${checkedCount}/${todayHabits.length})`}>
          {todayHabits.map(it => <HabitRow key={it.id} item={it} onCheck={onCheck} />)}
        </Section>
      )}

      {holiday && (
        <div style={{ fontSize: 12, color: 'var(--text-muted)', textAlign: 'center', marginTop: 4, marginBottom: 16 }}>
          🇰🇷 공휴일이라 주중 전용 습관은 표시되지 않아요
        </div>
      )}

      {/* This week */}
      {dueThisWeek.length > 0 && (
        <Section title="📅 이번 주 마감">
          {dueThisWeek
            .sort((a, b) => (a.days_left ?? 99) - (b.days_left ?? 99))
            .map(it => <TodoRow key={it.id} item={it} onToggle={onToggleDone} />)}
        </Section>
      )}

      {overdue.length === 0 && dueToday.length === 0 && dueThisWeek.length === 0 && todayHabits.length === 0 && (
        <div className="empty" style={{ marginTop: 40 }}>
          이번 주 일정이 없어요 🎉
        </div>
      )}
    </div>
  )
}

function Section({ title, accent, children }: { title: string; accent?: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 20 }}>
      <div style={{
        fontSize: 12, fontWeight: 600,
        color: accent ?? 'var(--text-muted)',
        textTransform: 'uppercase', letterSpacing: '0.05em',
        marginBottom: 8, paddingLeft: 4
      }}>
        {title}
      </div>
      {children}
    </div>
  )
}

function TodoRow({ item, onToggle }: { item: BriefingItem; onToggle: (item: BriefingItem) => void }) {
  return (
    <div className="card" style={{ marginBottom: 8 }}>
      <div className="item-header">
        <button className="item-check" onClick={() => onToggle(item)} />
        <div style={{ flex: 1 }}>
          <div className="item-title">
            {item.icon && <span style={{ marginRight: 5 }}>{item.icon}</span>}
            {item.title}
          </div>
          {item.note && <div className="item-note">{item.note}</div>}
        </div>
        {item.days_left !== undefined && item.days_left !== null && (
          <span className="badge" style={{
            background: item.days_left < 0 ? 'rgba(224,82,82,.2)' : item.days_left === 0 ? 'rgba(224,82,82,.15)' : 'rgba(245,166,35,.15)',
            color: item.days_left <= 0 ? 'var(--red)' : 'var(--yellow)',
            fontWeight: 700
          }}>
            {item.days_left < 0 ? `D+${-item.days_left}` : item.days_left === 0 ? 'D-Day' : `D-${item.days_left}`}
          </span>
        )}
      </div>
    </div>
  )
}

function HabitRow({ item, onCheck }: { item: BriefingItem; onCheck: (item: BriefingItem) => void }) {
  return (
    <div className="card" style={{ marginBottom: 8, opacity: item.checked_today ? 0.6 : 1 }}>
      <div className="item-header">
        <button
          className={`item-check habit-check${item.checked_today ? ' done' : ''}`}
          onClick={() => onCheck(item)}
        >
          {item.checked_today && <span style={{ color: '#fff', fontSize: 11, fontWeight: 700 }}>✓</span>}
        </button>
        <div style={{ flex: 1 }}>
          <div className="item-title" style={{ textDecoration: item.checked_today ? 'line-through' : 'none' }}>
            {item.icon && <span style={{ marginRight: 5 }}>{item.icon}</span>}
            {item.title}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          {item.streak > 0 && <span className="streak">🔥 {item.streak}</span>}
          {item.weekly_goal > 0 && (
            <span className="badge" style={{
              background: (item.week_count ?? 0) >= item.weekly_goal ? 'rgba(80,200,120,.15)' : 'var(--bg-elevated)',
              color: (item.week_count ?? 0) >= item.weekly_goal ? 'var(--green)' : 'var(--text-muted)'
            }}>
              주 {item.week_count ?? 0}/{item.weekly_goal}
            </span>
          )}
          <span className="badge">{item.reminder_hour}:{String(item.reminder_minute ?? 0).padStart(2, '0')}</span>
        </div>
      </div>
    </div>
  )
}

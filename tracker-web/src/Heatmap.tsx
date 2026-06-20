import { useEffect, useState } from 'react'

interface Props {
  itemId: number
}

export default function Heatmap({ itemId }: Props) {
  const [checkedDates, setCheckedDates] = useState<Set<string>>(new Set())

  useEffect(() => {
    fetch(`/api/items/${itemId}/heatmap`)
      .then(r => r.json())
      .then((dates: string[]) => setCheckedDates(new Set(dates)))
      .catch(() => {})
  }, [itemId])

  const fmtDate = (d: Date) => {
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    return `${y}-${m}-${day}`
  }

  // Build 84 days grid (12 weeks), most recent on right
  const days: string[] = []
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  for (let i = 83; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    days.push(fmtDate(d))
  }

  // pad to full weeks (Mon=0)
  const firstDow = (new Date(days[0]).getDay() + 6) % 7
  const padded = Array(firstDow).fill(null).concat(days)

  const weeks: (string | null)[][] = []
  for (let i = 0; i < padded.length; i += 7) {
    weeks.push(padded.slice(i, i + 7))
  }

  const todayStr = fmtDate(today)

  return (
    <div style={{ marginTop: 12, overflowX: 'auto' }}>
      <div style={{ display: 'flex', gap: 3 }}>
        {weeks.map((week, wi) => (
          <div key={wi} style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
            {week.map((day, di) => {
              if (!day) return <div key={di} style={{ width: 11, height: 11 }} />
              const checked = checkedDates.has(day)
              const isToday = day === todayStr
              return (
                <div
                  key={day}
                  title={day}
                  style={{
                    width: 11,
                    height: 11,
                    borderRadius: 2,
                    background: checked ? 'var(--accent)' : 'var(--bg-elevated)',
                    border: isToday ? '1px solid var(--accent)' : '1px solid transparent',
                    opacity: checked ? 1 : 0.5,
                  }}
                />
              )
            })}
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 6, fontSize: 10, color: 'var(--text-muted)' }}>
        <div style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--bg-elevated)', opacity: 0.5 }} />
        미체크
        <div style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--accent)' }} />
        체크 완료
      </div>
    </div>
  )
}

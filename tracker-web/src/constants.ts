export const TAG_COLORS: Record<string, string> = {
  '운동': '#3b82f6', '건강': '#22c55e', '공부': '#a855f7', '업무': '#f59e0b',
  '식사': '#ef4444', '청소': '#06b6d4', '쇼핑': '#ec4899', '사교': '#f97316',
  '취미': '#8b5cf6', '재정': '#10b981',
}

export function tagColor(tag: string): string {
  return TAG_COLORS[tag] ?? '#6b7280'
}

export const REMIND_OPTIONS = [
  { value: 0,   label: '알림 없음' },
  { value: 1,   label: '1분마다' },
  { value: 5,   label: '5분마다' },
  { value: 10,  label: '10분마다' },
  { value: 15,  label: '15분마다' },
  { value: 30,  label: '30분마다' },
  { value: 60,  label: '1시간마다' },
  { value: 120, label: '2시간마다' },
  { value: 240, label: '4시간마다' },
]

export const REMIND_BEFORE_OPTIONS = [
  { value: 0,  label: '없음' },
  { value: 5,  label: '5분 전' },
  { value: 10, label: '10분 전' },
  { value: 15, label: '15분 전' },
  { value: 30, label: '30분 전' },
  { value: 60, label: '1시간 전' },
]

export const HOUR_OPTIONS = Array.from({ length: 24 }, (_, i) => ({
  value: i,
  label: `${i}시`,
}))

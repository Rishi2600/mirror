// Lists are edited as comma-separated text.
export const toText = (items: string[]) => items.join(', ')
export const toList = (text: string) =>
  text
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

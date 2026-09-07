export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : 'Đã có lỗi xảy ra'
}

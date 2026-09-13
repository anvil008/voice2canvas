export function ExtendedWarning({ message }: { message: string }) {
  return <span className="extended-warning" role="status">{message}</span>;
}

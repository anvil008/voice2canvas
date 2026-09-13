/** Time-aware hero greetings. Exported pure for tests. */

const NAME = "Anvil";

interface GreetingBucket {
  from: number; // inclusive hour
  to: number; // exclusive hour
  lines: string[];
}

const BUCKETS: GreetingBucket[] = [
  {
    from: 5,
    to: 12,
    lines: [
      `Good morning, ${NAME} — what are we going to build today?`,
      `Morning, ${NAME}. What's first on the canvas?`,
    ],
  },
  {
    from: 12,
    to: 17,
    lines: [
      `Good afternoon, ${NAME} — what shall we build?`,
      `Afternoon, ${NAME}. What are we putting up next?`,
    ],
  },
  {
    from: 17,
    to: 22,
    lines: [
      `Good evening, ${NAME} — what are we making tonight?`,
      `Evening, ${NAME}. What's on your mind?`,
    ],
  },
  {
    from: 22,
    to: 5,
    lines: [
      `Midnight work, ${NAME}?`,
      `Late night, ${NAME} — what are we building?`,
    ],
  },
];

export function greetingForHour(hour: number, dayOfYear: number): string {
  const bucket =
    BUCKETS.find((b) =>
      b.from < b.to ? hour >= b.from && hour < b.to : hour >= b.from || hour < b.to,
    ) ?? BUCKETS[0];
  // Deterministic per day so the line doesn't change between re-renders,
  // but varies day to day.
  return bucket.lines[dayOfYear % bucket.lines.length];
}

export function currentGreeting(now: Date = new Date()): string {
  const start = new Date(now.getFullYear(), 0, 0);
  const dayOfYear = Math.floor((now.getTime() - start.getTime()) / 86_400_000);
  return greetingForHour(now.getHours(), dayOfYear);
}

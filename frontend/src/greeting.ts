/** Time-aware hero greetings. Exported pure for tests. */

interface GreetingTemplate {
  withName: string;
  withoutName: string;
}

interface GreetingBucket {
  from: number; // inclusive hour
  to: number; // exclusive hour
  templates: GreetingTemplate[];
}

const BUCKETS: GreetingBucket[] = [
  {
    from: 5,
    to: 12,
    templates: [
      {
        withName: "Good morning, {name} — what are we going to build today?",
        withoutName: "Good morning — what are we going to build today?",
      },
      {
        withName: "Morning, {name}. What's first on the canvas?",
        withoutName: "Morning. What's first on the canvas?",
      },
    ],
  },
  {
    from: 12,
    to: 17,
    templates: [
      {
        withName: "Good afternoon, {name} — what shall we build?",
        withoutName: "Good afternoon — what shall we build?",
      },
      {
        withName: "Afternoon, {name}. What are we putting up next?",
        withoutName: "Afternoon. What are we putting up next?",
      },
    ],
  },
  {
    from: 17,
    to: 22,
    templates: [
      {
        withName: "Good evening, {name} — what are we making tonight?",
        withoutName: "Good evening — what are we making tonight?",
      },
      {
        withName: "Evening, {name}. What's on your mind?",
        withoutName: "Evening. What's on your mind?",
      },
    ],
  },
  {
    from: 22,
    to: 5,
    templates: [
      {
        withName: "Midnight work, {name}?",
        withoutName: "Midnight work?",
      },
      {
        withName: "Late night, {name} — what are we building?",
        withoutName: "Late night — what are we building?",
      },
    ],
  },
];

function getStoredName(): string | null {
  try {
    if (typeof localStorage !== "undefined") {
      const val = localStorage.getItem("voice2canvas_name");
      return val && val.trim() ? val.trim() : null;
    }
  } catch {
    // Ignore storage access errors.
  }
  return null;
}

export function greetingForHour(hour: number, dayOfYear: number, name?: string | null): string {
  const bucket =
    BUCKETS.find((b) =>
      b.from < b.to ? hour >= b.from && hour < b.to : hour >= b.from || hour < b.to,
    ) ?? BUCKETS[0];
  const template = bucket.templates[dayOfYear % bucket.templates.length];
  const resolvedName = name !== undefined ? (name?.trim() || null) : getStoredName();
  if (resolvedName) {
    return template.withName.replace("{name}", resolvedName);
  }
  return template.withoutName;
}

export function currentGreeting(now: Date = new Date(), name?: string | null): string {
  const start = new Date(now.getFullYear(), 0, 0);
  const dayOfYear = Math.floor((now.getTime() - start.getTime()) / 86_400_000);
  return greetingForHour(now.getHours(), dayOfYear, name);
}

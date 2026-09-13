import type { TaskState } from "../protocol";
import { SparkIcon } from "../ui/icons";

export function PendingCard({
  status,
  title,
}: {
  status?: TaskState;
  title?: string;
}) {
  const caption = status === "researching" ? "researching…" : "generating…";

  return (
    <div className="pending-card" role="status" aria-label={`${title ?? "New card"} ${caption}`}>
      <div className="pending-card__heading">
        <span className="pending-card__spark" aria-hidden="true"><SparkIcon /></span>
        <div>
          <h3>{title ?? "Building your card"}</h3>
          <p>{caption}</p>
        </div>
      </div>
      <div className="pending-card__shimmer" aria-hidden="true">
        <span />
        <span />
        <span />
      </div>
    </div>
  );
}

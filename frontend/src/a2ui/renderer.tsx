import {
  Component,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ErrorInfo,
  type ReactNode,
} from "react";
import { renderMarkdown } from "@a2ui/markdown-it";
import "../styles/hero.css";
import { currentGreeting } from "../greeting";
import {
  A2uiSurface,
  basicCatalog,
  MarkdownContext,
  type ReactComponentImplementation,
} from "@a2ui/react/v0_9";
import {
  A2uiMessageListSchema,
  Catalog,
  MessageProcessor,
  type A2uiClientAction,
  type A2uiMessage,
  type SurfaceModel,
} from "@a2ui/web_core/v0_9";
import type { LayoutSlot, TaskState } from "../protocol";
import {
  EXTENDED_CATALOG_ID,
  EXTENDED_WARNING_COMPONENT,
  extendedComponentImplementations,
  validateExtendedComponent,
} from "./extended/catalog";
import { PendingCard } from "./PendingCard";

/** The catalog ID mandated by PROTOCOL.md for A2UI v0.9.1 frames. */
export const PROTOCOL_BASIC_CATALOG_ID =
  "https://a2ui.org/specification/v0_9_1/catalogs/basic/catalog.json";

/**
 * The currently published React basic catalog still carries the v0_9 path. Clone
 * its components under the backend's v0.9.1 catalog ID so MessageProcessor accepts
 * protocol-compliant createSurface frames without rewriting server data.
 */
const protocolBasicCatalog = new Catalog<ReactComponentImplementation>(
  PROTOCOL_BASIC_CATALOG_ID,
  [...basicCatalog.components.values()],
  [...basicCatalog.functions.values()],
  basicCatalog.themeSchema,
);

/** The extended catalog is additive: the entire basic catalog remains valid. */
const extendedCatalog = new Catalog<ReactComponentImplementation>(
  EXTENDED_CATALOG_ID,
  [...basicCatalog.components.values(), ...extendedComponentImplementations],
  [...basicCatalog.functions.values()],
  basicCatalog.themeSchema,
);

const basicComponentNames = new Set(basicCatalog.components.keys());

function sanitizeExtendedComponents(message: A2uiMessage): A2uiMessage {
  if (!("updateComponents" in message)) return message;
  return {
    ...message,
    updateComponents: {
      ...message.updateComponents,
      components: message.updateComponents.components.map((component) => {
        const { id, component: componentName, ...properties } = component;
        if (basicComponentNames.has(componentName)) return component;
        const warning = validateExtendedComponent(componentName, properties);
        if (!warning) return component;
        return {
          id,
          component: EXTENDED_WARNING_COMPONENT,
          message: warning,
        };
      }),
    },
  } as A2uiMessage;
}

export interface UpstreamAction {
  context: Record<string, unknown>;
  name: string;
  sourceComponentId: string;
  surfaceId: string;
}

/**
 * Isolated A2UI adapter: it validates protocol messages, owns MessageProcessor,
 * and exposes only feed/getSurface/subscribe to the rest of the dashboard.
 */
export class A2uiRuntime {
  private readonly deletedSubscription;
  private readonly listeners = new Set<() => void>();
  private readonly processor: MessageProcessor<ReactComponentImplementation>;
  private revision = 0;
  private readonly createdSubscription;
  private readonly extendedSurfaceIds = new Set<string>();

  constructor(private readonly onAction: (action: UpstreamAction) => void) {
    this.processor = new MessageProcessor(
      [protocolBasicCatalog, extendedCatalog],
      (action) => this.forwardAction(action),
      { version: "v0.9.1" },
    );
    this.createdSubscription = this.processor.onSurfaceCreated(() => this.notify());
    this.deletedSubscription = this.processor.onSurfaceDeleted(() => this.notify());
  }

  feed(messages: unknown[]): void {
    const result = A2uiMessageListSchema.safeParse(messages);
    if (!result.success) {
      throw new Error(`A2UI message validation failed: ${result.error.message}`);
    }
    for (const message of result.data) {
      if ("createSurface" in message) {
        const { surfaceId, catalogId } = message.createSurface;
        if (this.processor.model.getSurface(surfaceId)) {
          this.processor.model.deleteSurface(surfaceId);
        }
        this.extendedSurfaceIds.delete(surfaceId);
        if (catalogId === EXTENDED_CATALOG_ID) {
          this.extendedSurfaceIds.add(surfaceId);
        }
      }
      const surfaceId =
        "updateComponents" in message
          ? message.updateComponents.surfaceId
          : "updateDataModel" in message
            ? message.updateDataModel.surfaceId
            : undefined;
      const processed = surfaceId && this.extendedSurfaceIds.has(surfaceId)
        ? sanitizeExtendedComponents(message)
        : message;
      this.processor.processMessages([processed]);
      if ("deleteSurface" in message) {
        this.extendedSurfaceIds.delete(message.deleteSurface.surfaceId);
      }
    }
    this.notify();
  }

  reset(): void {
    for (const surfaceId of this.processor.model.surfacesMap.keys()) {
      this.processor.model.deleteSurface(surfaceId);
    }
    this.extendedSurfaceIds.clear();
    this.notify();
  }

  getSurface(surfaceId: string): SurfaceModel<ReactComponentImplementation> | undefined {
    return this.processor.model.getSurface(surfaceId);
  }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = (): number => this.revision;

  dispose(): void {
    this.createdSubscription.unsubscribe();
    this.deletedSubscription.unsubscribe();
    this.processor.model.dispose();
    this.extendedSurfaceIds.clear();
    this.listeners.clear();
  }

  private forwardAction(action: A2uiClientAction): void {
    // A2UI's internal action contains a timestamp. PROTOCOL.md intentionally
    // defines the upstream action shape without that extra field.
    this.onAction({
      context: action.context,
      name: action.name,
      sourceComponentId: action.sourceComponentId,
      surfaceId: action.surfaceId,
    });
  }

  private notify(): void {
    this.revision += 1;
    for (const listener of this.listeners) {
      listener();
    }
  }
}

class SurfaceErrorBoundary extends Component<
  { children: ReactNode; revision: number },
  { error?: string; revision: number }
> {
  state = { error: undefined, revision: this.props.revision };

  static getDerivedStateFromError(error: unknown) {
    return { error: error instanceof Error ? error.message : String(error) };
  }

  static getDerivedStateFromProps(
    props: { revision: number },
    state: { error?: string; revision: number },
  ) {
    return props.revision === state.revision
      ? null
      : { error: undefined, revision: props.revision };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("A2UI surface render failed", error, info.componentStack);
  }

  render() {
    return this.state.error
      ? <span className="extended-warning" role="status">Malformed component props</span>
      : this.props.children;
  }
}

export interface PendingSurface {
  surfaceId: string;
  taskId: string;
  status?: TaskState;
  title?: string;
}

export function A2uiCanvas({
  pendingCards = [],
  runtime,
  slots,
}: {
  pendingCards?: PendingSurface[];
  runtime: A2uiRuntime;
  slots: LayoutSlot[];
}) {
  const runtimeRevision = useSyncExternalStore(
    runtime.subscribe,
    runtime.getSnapshot,
    runtime.getSnapshot,
  );
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const slotRectsRef = useRef(new Map<string, DOMRect>());
  const pendingSurfaceIds = useMemo(
    () => new Set(pendingCards.map((pending) => pending.surfaceId)),
    [pendingCards],
  );
  const displayedSlots = useMemo(
    () => slots.filter((slot) => !pendingSurfaceIds.has(slot.surfaceId)),
    [pendingSurfaceIds, slots],
  );
  const visualSlotsRef = useRef<Array<{ slot: LayoutSlot; exiting: boolean }>>(
    displayedSlots.map((slot) => ({ slot, exiting: false })),
  );
  const removalTimers = useRef(new Map<string, number>());
  const [visualSlots, setVisualSlots] = useState(visualSlotsRef.current);
  const visualPendingRef = useRef<Array<{ pending: PendingSurface; exiting: boolean }>>(
    pendingCards.map((pending) => ({ pending, exiting: false })),
  );
  const pendingRemovalTimers = useRef(new Map<string, number>());
  const [visualPending, setVisualPending] = useState(visualPendingRef.current);

  useEffect(() => {
    const incoming = new Map(displayedSlots.map((slot) => [slot.surfaceId, slot]));
    const previousIds = new Set(visualSlotsRef.current.map(({ slot }) => slot.surfaceId));
    const next = visualSlotsRef.current.map(({ slot }) => {
      const updated = incoming.get(slot.surfaceId);
      return updated ? { slot: updated, exiting: false } : { slot, exiting: true };
    });

    for (const slot of displayedSlots) {
      const timer = removalTimers.current.get(slot.surfaceId);
      if (timer !== undefined) {
        window.clearTimeout(timer);
        removalTimers.current.delete(slot.surfaceId);
      }
      if (!previousIds.has(slot.surfaceId)) next.push({ slot, exiting: false });
    }

    for (const item of next) {
      if (!item.exiting || removalTimers.current.has(item.slot.surfaceId)) continue;
      const timer = window.setTimeout(() => {
        removalTimers.current.delete(item.slot.surfaceId);
        visualSlotsRef.current = visualSlotsRef.current.filter(
          ({ slot }) => slot.surfaceId !== item.slot.surfaceId,
        );
        setVisualSlots(visualSlotsRef.current);
      }, 320);
      removalTimers.current.set(item.slot.surfaceId, timer);
    }

    next.sort((left, right) => left.slot.order - right.slot.order);
    // Skip the state write when nothing changed: props like `slots` may be a
    // fresh array reference on every render, and an unconditional setState
    // here would loop the render cycle forever.
    const unchanged =
      next.length === visualSlotsRef.current.length &&
      next.every((item, index) => {
        const previous = visualSlotsRef.current[index];
        return (
          previous.slot.surfaceId === item.slot.surfaceId &&
          previous.slot.order === item.slot.order &&
          previous.slot.span === item.slot.span &&
          previous.exiting === item.exiting
        );
      });
    if (unchanged) return;
    visualSlotsRef.current = next;
    setVisualSlots(next);
  }, [displayedSlots]);

  useEffect(() => {
    const incoming = new Map(pendingCards.map((pending) => [pending.surfaceId, pending]));
    const replacementIds = new Set(displayedSlots.map((slot) => slot.surfaceId));
    const previousIds = new Set(
      visualPendingRef.current.map(({ pending }) => pending.surfaceId),
    );
    const next = visualPendingRef.current.flatMap(({ pending }) => {
      const updated = incoming.get(pending.surfaceId);
      if (updated) return [{ pending: updated, exiting: false }];
      if (replacementIds.has(pending.surfaceId)) {
        const timer = pendingRemovalTimers.current.get(pending.surfaceId);
        if (timer !== undefined) window.clearTimeout(timer);
        pendingRemovalTimers.current.delete(pending.surfaceId);
        return [];
      }
      return [{ pending, exiting: true }];
    });

    for (const pending of pendingCards) {
      const timer = pendingRemovalTimers.current.get(pending.surfaceId);
      if (timer !== undefined) {
        window.clearTimeout(timer);
        pendingRemovalTimers.current.delete(pending.surfaceId);
      }
      if (!previousIds.has(pending.surfaceId)) next.push({ pending, exiting: false });
    }

    for (const item of next) {
      const surfaceId = item.pending.surfaceId;
      if (!item.exiting || pendingRemovalTimers.current.has(surfaceId)) continue;
      const timer = window.setTimeout(() => {
        pendingRemovalTimers.current.delete(surfaceId);
        visualPendingRef.current = visualPendingRef.current.filter(
          ({ pending }) => pending.surfaceId !== surfaceId,
        );
        setVisualPending(visualPendingRef.current);
      }, 320);
      pendingRemovalTimers.current.set(surfaceId, timer);
    }

    // Same guard as the slots effect: only write state on real changes so
    // fresh prop references cannot re-trigger the render loop.
    const unchanged =
      next.length === visualPendingRef.current.length &&
      next.every((item, index) => {
        const previous = visualPendingRef.current[index];
        return (
          previous.pending.surfaceId === item.pending.surfaceId &&
          previous.pending.taskId === item.pending.taskId &&
          previous.pending.status === item.pending.status &&
          previous.pending.title === item.pending.title &&
          previous.exiting === item.exiting
        );
      });
    if (unchanged) return;
    visualPendingRef.current = next;
    setVisualPending(next);
  }, [displayedSlots, pendingCards]);

  useLayoutEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      slotRectsRef.current.clear();
      return;
    }

    const nextRects = new Map<string, DOMRect>();
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
    for (const element of canvas.querySelectorAll<HTMLElement>(".surface-slot[data-surface-id]")) {
      const surfaceId = element.dataset.surfaceId;
      if (!surfaceId) continue;
      const nextRect = element.getBoundingClientRect();
      nextRects.set(surfaceId, nextRect);
      const previousRect = slotRectsRef.current.get(surfaceId);
      if (!previousRect || reduceMotion || typeof element.animate !== "function") continue;

      const deltaX = previousRect.left - nextRect.left;
      const deltaY = previousRect.top - nextRect.top;
      const scaleX = nextRect.width > 0 ? previousRect.width / nextRect.width : 1;
      const scaleY = nextRect.height > 0 ? previousRect.height / nextRect.height : 1;
      if (
        Math.abs(deltaX) < 0.5 &&
        Math.abs(deltaY) < 0.5 &&
        Math.abs(scaleX - 1) < 0.01 &&
        Math.abs(scaleY - 1) < 0.01
      ) continue;

      element.animate(
        [
          {
            transform: `translate(${deltaX}px, ${deltaY}px) scale(${scaleX}, ${scaleY})`,
            transformOrigin: "top left",
          },
          { transform: "none", transformOrigin: "top left" },
        ],
        { duration: 320, easing: "cubic-bezier(0.2, 0.75, 0.25, 1)" },
      );
    }
    slotRectsRef.current = nextRects;
  }, [visualPending, visualSlots]);

  useEffect(
    () => () => {
      for (const timer of removalTimers.current.values()) window.clearTimeout(timer);
      removalTimers.current.clear();
      for (const timer of pendingRemovalTimers.current.values()) window.clearTimeout(timer);
      pendingRemovalTimers.current.clear();
    },
    [],
  );

  return (
    <MarkdownContext.Provider value={renderMarkdown}>
      {visualSlots.length === 0 && visualPending.length === 0 ? (
        <div className="idle-hero">
          <div className="idle-copy">
            <h1>{currentGreeting()}</h1>
          </div>
        </div>
      ) : (
        <div
          className="a2ui-canvas"
          data-density={
            visualSlots.length + visualPending.length <= 1
              ? "solo"
              : visualSlots.length + visualPending.length === 2
                ? "sparse"
                : "normal"
          }
          ref={canvasRef}
        >
          {visualSlots.map(({ slot, exiting }) => {
            const surface = runtime.getSurface(slot.surfaceId);
            return (
              <section
                className={`surface-slot${slot.span === 2 ? " surface-slot--span-2" : ""}${surface?.catalog.id === EXTENDED_CATALOG_ID ? " surface-slot--extended" : ""}${exiting || !surface ? " surface-slot--exiting" : ""}`}
                key={slot.surfaceId}
                style={{ order: slot.order }}
                data-surface-id={slot.surfaceId}
                data-span={slot.span ?? 1}
                data-catalog-id={surface?.catalog.id}
              >
                {surface ? (
                  <SurfaceErrorBoundary revision={runtimeRevision}>
                    <A2uiSurface surface={surface} />
                  </SurfaceErrorBoundary>
                ) : (
                  <div className="surface-removing" aria-label="Removing card">
                    <span />
                    <span />
                  </div>
                )}
              </section>
            );
          })}
          {visualPending.map(({ pending, exiting }) => (
            <section
              className={`surface-slot surface-slot--pending${exiting ? " surface-slot--exiting" : ""}`}
              key={`pending:${pending.surfaceId}`}
              data-surface-id={pending.surfaceId}
              data-task-id={pending.taskId}
            >
              <PendingCard status={pending.status} title={pending.title} />
            </section>
          ))}
        </div>
      )}
    </MarkdownContext.Provider>
  );
}

export type { A2uiMessage };

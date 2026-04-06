import type { PropsWithChildren, ReactNode } from "react";

export function ShellFrame(props: {
  routeLabel: string;
  title: string;
  subtitle: string;
  nav: Array<{ label: string; href: string }>;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const { routeLabel, title, subtitle, nav, actions, children } = props;
  return (
    <div className="app-shell">
      <aside className="side-rail">
        <div className="brand-lockup">
          <div className="brand-mark">CR</div>
          <div>
            <p className="brand-kicker">{routeLabel}</p>
            <h1>Codex Remote</h1>
          </div>
        </div>
        <p className="side-copy">{subtitle}</p>
        <nav className="side-nav" aria-label="Page Sections">
          {nav.map((item) => (
            <a key={item.href} href={item.href}>
              {item.label}
            </a>
          ))}
        </nav>
      </aside>
      <main className="main-stage">
        <header className="page-hero">
          <div>
            <p className="page-kicker">{routeLabel}</p>
            <h2>{title}</h2>
          </div>
          {actions ? <div className="hero-actions">{actions}</div> : null}
        </header>
        {children}
      </main>
    </div>
  );
}

export function Panel(props: PropsWithChildren<{ id?: string; title: string; description?: string; className?: string }>) {
  const { id, title, description, className, children } = props;
  return (
    <section id={id} className={`panel${className ? ` ${className}` : ""}`}>
      <div className="panel-head">
        <div>
          <h3>{title}</h3>
          {description ? <p>{description}</p> : null}
        </div>
      </div>
      {children}
    </section>
  );
}

export function StatGrid(props: PropsWithChildren) {
  return <div className="stat-grid">{props.children}</div>;
}

export function StatCard(props: { label: string; value: string | number; detail?: string; tone?: "default" | "accent" | "warn" }) {
  return (
    <div className={`stat-card${props.tone ? ` ${props.tone}` : ""}`}>
      <p>{props.label}</p>
      <strong>{props.value}</strong>
      {props.detail ? <span>{props.detail}</span> : null}
    </div>
  );
}

export function StatusBadge(props: { value: string; tone?: "neutral" | "good" | "warn" | "danger" }) {
  return <span className={`status-badge ${props.tone ?? "neutral"}`}>{props.value}</span>;
}

export function DefinitionList(props: { items: Array<{ label: string; value: ReactNode }> }) {
  return (
    <dl className="definition-list">
      {props.items.map((item) => (
        <div key={item.label}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function DataList(props: { items: Array<{ title: string; meta?: string; detail?: string; tone?: "neutral" | "good" | "warn" | "danger" }> }) {
  return (
    <div className="data-list">
      {props.items.map((item) => (
        <article key={`${item.title}-${item.meta ?? ""}`} className="data-row">
          <div>
            <h4>{item.title}</h4>
            {item.detail ? <p>{item.detail}</p> : null}
          </div>
          <div className="data-meta">
            {item.meta ? <span>{item.meta}</span> : null}
            {item.tone ? <StatusBadge value={toneLabel(item.tone)} tone={item.tone} /> : null}
          </div>
        </article>
      ))}
    </div>
  );
}

export function LoadingState(props: { title: string; description?: string }) {
  return (
    <Panel title={props.title} description={props.description}>
      <div className="empty-state">
        <div className="loading-dot" />
        <span>正在读取最新状态</span>
      </div>
    </Panel>
  );
}

export function ErrorState(props: { title: string; description?: string; detail: string }) {
  return (
    <Panel title={props.title} description={props.description}>
      <div className="empty-state error">
        <strong>加载失败</strong>
        <p>{props.detail}</p>
      </div>
    </Panel>
  );
}

function toneLabel(tone: "neutral" | "good" | "warn" | "danger"): string {
  switch (tone) {
    case "good":
      return "Healthy";
    case "warn":
      return "Attention";
    case "danger":
      return "Blocked";
    default:
      return "Info";
  }
}

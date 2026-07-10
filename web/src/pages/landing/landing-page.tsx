import {
  ArrowRight,
  CheckCircle2,
  CircleDot,
  Clock3,
  Compass,
  ListTodo,
} from "lucide-react";
import { Link } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";

import "./landing-page.css";
import {
  capabilityItems,
  flowItems,
  heroActivityItems,
  heroContextItems,
  heroRouteNodes,
  ledgerItems,
} from "./landing-demo-data";
import { WorkbenchSection } from "./landing-workbench";

function LandingHeader() {
  return (
    <header className="landing-header">
      <div className="landing-header-inner">
        <Link aria-label="Nexus home" className="landing-brand" to={APP_ROUTE_PATHS.landing}>
          <img alt="" className="landing-brand-logo" src="/logo.webp" />
          <span>NEXUS</span>
        </Link>

        <nav aria-label="Landing navigation" className="landing-nav">
          <a href="#workbench">Workbench</a>
          <a href="#flow">Flow</a>
          <a href="#capabilities">Capabilities</a>
          <a href="#control">Control</a>
        </nav>

        <div className="landing-actions">
          <Link className="landing-primary-button" to={APP_ROUTE_PATHS.launcher}>
            Enter app
            <ArrowRight size={15} />
          </Link>
        </div>
      </div>
    </header>
  );
}

function HeroSignal() {
  return (
    <aside className="landing-hero-signal" aria-label="Nexus routing overview">
      <div className="landing-hero-route" aria-label="Nexus execution route">
        {heroRouteNodes.map(([title, copy]) => (
          <div className="landing-hero-route-node" key={title}>
            <span aria-hidden="true" />
            <strong>{title}</strong>
            <p>{copy}</p>
          </div>
        ))}
      </div>

      <div className="landing-hero-activity" aria-label="Live Nexus task activity">
        <div className="landing-hero-activity-head">
          <span>Active task</span>
          <strong>Landing review</strong>
        </div>
        {heroActivityItems.map(([time, actor, copy]) => (
          <div className="landing-hero-activity-row" key={`${time}-${actor}`}>
            <span>{time}</span>
            <strong>{actor}</strong>
            <p>{copy}</p>
          </div>
        ))}
      </div>

      <div className="landing-hero-context" aria-label="Nexus shared context">
        {heroContextItems.map((item) => (
          <span key={item}>{item}</span>
        ))}
      </div>
    </aside>
  );
}

function HeroSection() {
  return (
    <section className="landing-hero">
      <div className="landing-section landing-hero-inner">
        <div className="landing-hero-copy-block">
          <div className="landing-hero-title-wrap">
            <h1>Nexus</h1>
            <img alt="" aria-hidden="true" className="landing-hero-title-persona" src="/nexus/stickers/card-top.png" />
          </div>
          <p className="landing-hero-line">Agent work, in one calm workspace.</p>
          <p className="landing-hero-copy">
            Rooms, DMs, skills, connectors, runtime-managed memory, schedules, and workspace files share one operating surface.
          </p>
        </div>
        <HeroSignal />
      </div>
    </section>
  );
}

function FlowSection() {
  return (
    <section className="landing-section landing-flow-section" id="flow">
      <div className="landing-section-heading-row">
        <h2>From prompt to persistent work.</h2>
        <p>A short route from the launcher into durable agent execution.</p>
      </div>

      <div className="landing-flow-grid">
        {flowItems.map(([step, title, copy]) => (
          <article className="landing-flow-item" key={step}>
            <span>{step}</span>
            <h3>{title}</h3>
            <p>{copy}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

function CapabilitiesSection() {
  return (
    <section className="landing-section landing-capabilities" id="capabilities">
      <div className="landing-section-heading-row">
        <h2>The actual Nexus objects.</h2>
        <p>These are product modules, not generic feature names.</p>
      </div>

      <div className="landing-capability-list">
        {capabilityItems.map(({ title, copy, meta, Icon }) => (
          <article className="landing-capability-row" key={title}>
            <Icon size={18} />
            <strong>{title}</strong>
            <p>{copy}</p>
            <span>{meta}</span>
          </article>
        ))}
      </div>
    </section>
  );
}

function ControlSection() {
  return (
    <section className="landing-section landing-control" id="control">
      <div className="landing-control-copy">
        <h2>Fast agents. Visible boundaries.</h2>
        <p>
          Runtime state, permissions, memory files, and scheduled tasks stay close to the conversation so
          automation remains reviewable.
        </p>
        <div className="landing-control-checks">
          {[
            "Default ask mode for sensitive actions",
            "Plan-first execution for higher-risk work",
            "Workspace output next to the conversation",
            "Scheduled runs with history and delivery targets",
          ].map((item) => (
            <span key={item}>
              <CheckCircle2 size={15} />
              {item}
            </span>
          ))}
        </div>
      </div>

      <div className="landing-ledger">
        <div className="landing-ledger-head">
          <ListTodo size={17} />
          <strong>Run ledger</strong>
          <span>active</span>
        </div>
        {ledgerItems.map(([time, actor, action], index) => (
          <div className="landing-ledger-row" key={`${time}-${actor}`}>
            <span>{time}</span>
            {index === ledgerItems.length - 1 ? <CircleDot size={14} /> : <Clock3 size={14} />}
            <strong>{actor}</strong>
            <p>{action}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function FinalCta() {
  return (
    <section className="landing-final">
      <div className="landing-section landing-final-inner">
        <img alt="" src="/logo.webp" />
        <h2>Start from the launcher.</h2>
        <p>Route the task, keep the files, review the run.</p>
      </div>
    </section>
  );
}

function LandingFooter() {
  return (
    <footer className="landing-footer">
      <div className="landing-section landing-footer-inner">
        <div className="landing-footer-brand">
          <div className="landing-footer-mark">
            <img alt="" src="/logo.webp" />
          </div>
          <div>
            <strong>Nexus</strong>
            <p>Agent work, routed and reviewed in one workspace.</p>
          </div>
        </div>

        <div className="landing-footer-bottom">
          <a href="https://beian.miit.gov.cn/" rel="noreferrer" target="_blank">
            粤ICP备2023024185号-2
          </a>
        </div>
      </div>
    </footer>
  );
}

export function LandingPage() {
  return (
    <main className="landing-page">
      <LandingHeader />
      <HeroSection />
      <WorkbenchSection />
      <FlowSection />
      <CapabilitiesSection />
      <ControlSection />
      <FinalCta />
      <LandingFooter />
    </main>
  );
}

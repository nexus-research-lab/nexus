export function OperationStageMotionStyles() {
  return (
    <style>
      {`
        @keyframes nexus-operation-window-enter {
          0% { opacity: 0; transform: translate3d(0, 14px, 0) scale(0.985); filter: blur(3px); }
          100% { opacity: 1; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
        }

        @keyframes nexus-operation-window-float {
          0%, 100% {
            translate:
              var(--operation-window-drag-x, 0px)
              var(--operation-window-drag-y, 0px);
          }
          50% {
            translate:
              var(--operation-window-drag-x, 0px)
              calc(var(--operation-window-drag-y, 0px) - 3px);
          }
        }

        @keyframes nexus-operation-preview-line {
          0% { opacity: 0; transform: translateX(-8px); }
          100% { opacity: 1; transform: translateX(0); }
        }

        @keyframes nexus-operation-scan {
          0% { transform: translateY(-18px); opacity: 0; }
          12% { opacity: 0.85; }
          100% { transform: translateY(180px); opacity: 0; }
        }

        @keyframes nexus-operation-shimmer {
          0% { transform: translateX(-120%); }
          100% { transform: translateX(120%); }
        }

        @keyframes nexus-operation-caret {
          0%, 45% { opacity: 1; }
          46%, 100% { opacity: 0; }
        }

        @keyframes nexus-operation-type-line {
          0% { clip-path: inset(0 100% 0 0); }
          100% { clip-path: inset(0 0 0 0); }
        }

        @keyframes nexus-operation-editor-file-activity {
          0%, 100% { box-shadow: inset 0 1px 0 rgba(255,255,255,.05), 0 0 0 rgba(141,224,173,0); }
          50% { box-shadow: inset 0 1px 0 rgba(255,255,255,.08), 0 0 18px rgba(141,224,173,.12); }
        }

        @keyframes nexus-operation-pulse-width {
          0%, 100% { transform: scaleX(0.86); opacity: 0.7; }
          50% { transform: scaleX(1); opacity: 1; }
        }

        @keyframes nexus-operation-focus-dot {
          0%, 100% { transform: translate(-50%, -50%) scale(0.72); opacity: 0.52; }
          50% { transform: translate(-50%, -50%) scale(1.4); opacity: 1; }
        }

        @keyframes nexus-operation-scene-enter {
          0% {
            opacity: 0.12;
            transform:
              translate3d(
                var(--operation-scene-enter-x, 0),
                var(--operation-scene-enter-y, 14px),
                0
              )
              scale(.992);
            filter: blur(5px);
          }
          100% { opacity: 1; transform: scale(1); filter: blur(0); }
        }

        @keyframes nexus-operation-idle-exit {
          0% { opacity: 1; transform: scale(1); filter: blur(0); }
          46% { opacity: .68; filter: blur(.5px); }
          100% {
            opacity: 0;
            transform:
              translate3d(
                var(--operation-idle-exit-x, 0),
                var(--operation-idle-exit-y, 0),
                0
              )
              scale(var(--operation-idle-exit-scale, 1.035));
            filter: blur(var(--operation-idle-exit-blur, 4px));
          }
        }

        @keyframes nexus-operation-idle-particles-yield {
          0% { opacity: .94; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
          38% { opacity: .82; transform: translate3d(0, -2px, 0) scale(.99); filter: blur(.2px); }
          100% {
            opacity: 0;
            transform:
              translate3d(
                calc(var(--operation-idle-exit-x, 0) * .42),
                calc(var(--operation-idle-exit-y, 0) * .42),
                0
              )
              scale(.86);
            filter: blur(2.5px);
          }
        }

        @keyframes nexus-operation-idle-pulse {
          0%, 100% { opacity: .9; transform: translate3d(0, 0, 0) scale(1); }
          50% { opacity: 1; transform: translate3d(0, -2px, 0) scale(1.006); }
        }

        @keyframes nexus-operation-boot-signal {
          0% { opacity: 0; transform: translate3d(0, 12px, 0) scale(.985); filter: blur(4px); }
          42% { opacity: 1; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
          100% { opacity: .88; transform: translate3d(0, -4px, 0) scale(1.006); filter: blur(.2px); }
        }

        @keyframes nexus-operation-boot-line {
          0% { transform: scaleX(.12); opacity: .3; }
          48% { transform: scaleX(.76); opacity: .88; }
          100% { transform: scaleX(1); opacity: .72; }
        }

        @keyframes nexus-operation-event-signal {
          0% { opacity: 0; transform: translate3d(-50%, -10px, 0) scale(.985); filter: blur(3px); }
          20% { opacity: 1; transform: translate3d(-50%, 0, 0) scale(1); filter: blur(0); }
          78% { opacity: 1; transform: translate3d(-50%, 0, 0) scale(1); filter: blur(0); }
          100% { opacity: 0; transform: translate3d(-50%, -4px, 0) scale(1.006); filter: blur(.8px); }
        }

        @keyframes nexus-operation-materializing-signal {
          0% { opacity: 0; transform: translate3d(8px, -8px, 0) scale(.985); filter: blur(3px); }
          22% { opacity: 1; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
          100% { opacity: .92; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
        }

        @keyframes nexus-operation-materializing-line {
          0% { transform: scaleX(.18); opacity: .42; }
          55% { transform: scaleX(.82); opacity: .9; }
          100% { transform: scaleX(1); opacity: .78; }
        }

        @keyframes nexus-operation-window-restore {
          0% { opacity: .72; filter: saturate(.9) brightness(1.03); transform: translate3d(0, 54px, 0) scale(.86); }
          58% { opacity: 1; filter: saturate(1.04) brightness(1.01); transform: translate3d(0, -4px, 0) scale(1.012); }
          100% { filter: saturate(1) brightness(1); transform: translate3d(0, 0, 0) scale(1); }
        }

        @keyframes nexus-operation-dock-tile-enter {
          0% { opacity: 0; transform: translate3d(0, -18px, 0) scale(.82); filter: blur(2px); }
          62% { opacity: 1; transform: translate3d(0, 3px, 0) scale(1.06); filter: blur(0); }
          100% { opacity: 1; transform: translate3d(0, 0, 0) scale(1); filter: blur(0); }
        }

        .operation-stage-window {
          animation: nexus-operation-window-enter 420ms cubic-bezier(.18,.88,.24,1) both;
          animation-delay: var(--operation-delay, 0ms);
          transform-origin: 50% 60%;
        }

        .operation-stage-window-launch-dock {
          transform-origin: 50% 100%;
        }

        .operation-stage-window-launch-desktop {
          transform-origin: 88% 20%;
        }

        .operation-stage-window-focus {
          box-shadow:
            0 32px 82px rgba(34,48,72,.18),
            0 0 0 1px rgba(255,255,255,.72),
            0 0 24px rgba(91,114,255,.12);
        }

        .operation-stage-window-stage-manager {
          transform-origin: 0% 50%;
          box-shadow:
            0 16px 38px rgba(18,28,42,.12),
            0 0 0 1px rgba(255,255,255,.62);
        }

        .operation-stage-window-stage-manager:hover {
          filter: saturate(1.04) brightness(1.02);
          box-shadow:
            0 20px 46px rgba(18,28,42,.16),
            0 0 0 1px rgba(255,255,255,.76);
        }

        .operation-stage-window-dragging {
          animation-play-state: paused;
          box-shadow:
            0 36px 90px rgba(34,48,72,.22),
            0 0 0 1px rgba(255,255,255,.78),
            0 0 28px rgba(91,114,255,.14);
        }

        .operation-stage-window-restoring {
          animation: nexus-operation-window-restore 360ms cubic-bezier(.2,.9,.18,1) both;
          transform-origin: 50% 100%;
        }

        .operation-window-dock-minimized {
          animation: nexus-operation-dock-tile-enter 260ms cubic-bezier(.18,.88,.22,1) both;
          transform-origin: 50% 0%;
        }

        .operation-window-traffic-icon {
          opacity: 0;
          transform: scale(.72);
          transition:
            opacity 120ms ease,
            transform 120ms ease;
        }

        .operation-window-traffic:hover .operation-window-traffic-icon,
        .operation-window-traffic-button:focus-visible .operation-window-traffic-icon {
          opacity: 1;
          transform: scale(1);
        }

        .operation-preview-line {
          animation: nexus-operation-preview-line 320ms ease-out both;
          animation-delay: var(--operation-delay, 0ms);
        }

        .operation-scan-line {
          position: absolute;
          left: 0;
          right: 0;
          top: 42px;
          height: 1px;
          background: linear-gradient(90deg, transparent, rgba(91,114,255,.46), rgba(79,162,159,.36), transparent);
          animation: nexus-operation-scan 2.6s ease-in-out infinite;
        }

        .operation-stage-gridlines {
          background-image:
            linear-gradient(rgba(71,85,105,.055) 1px, transparent 1px),
            linear-gradient(90deg, rgba(71,85,105,.045) 1px, transparent 1px);
          background-size: 34px 34px;
          mask-image: radial-gradient(circle at 50% 45%, black, transparent 72%);
        }

        .operation-desktop-wallpaper {
          overflow: hidden;
          background:
            linear-gradient(118deg, transparent 0 21.2%, rgba(91,114,255,.074) 21.35% 22.55%, rgba(255,255,255,.26) 22.56% 22.82%, transparent 22.94% 100%),
            linear-gradient(142deg, transparent 0 57.5%, rgba(79,162,159,.066) 57.64% 58.72%, rgba(255,255,255,.24) 58.73% 59.02%, transparent 59.14% 100%),
            repeating-linear-gradient(90deg, rgba(32,43,58,.034) 0 1px, transparent 1px 88px),
            repeating-linear-gradient(0deg, rgba(32,43,58,.027) 0 1px, transparent 1px 88px),
            linear-gradient(145deg, rgba(252,254,255,.985) 0%, rgba(238,245,249,.94) 46%, rgba(223,234,238,.90) 100%);
        }

        .operation-desktop-wallpaper::before {
          content: "";
          position: absolute;
          inset: 8% 6% 12% 7%;
          background:
            linear-gradient(90deg, transparent 0 13%, rgba(28,42,60,.105) 13.04% 13.12%, transparent 13.18% 100%),
            linear-gradient(90deg, transparent 0 57%, rgba(91,114,255,.135) 57.04% 57.16%, transparent 57.25% 100%),
            linear-gradient(0deg, transparent 0 31%, rgba(79,162,159,.125) 31.04% 31.16%, transparent 31.25% 100%),
            linear-gradient(0deg, transparent 0 69%, rgba(28,42,60,.084) 69.04% 69.12%, transparent 69.20% 100%),
            linear-gradient(28deg, transparent 0 29%, rgba(91,114,255,.165) 29.04% 29.22%, transparent 29.34% 100%),
            linear-gradient(153deg, transparent 0 44%, rgba(79,162,159,.145) 44.04% 44.22%, transparent 44.34% 100%),
            repeating-linear-gradient(90deg, transparent 0 38px, rgba(255,255,255,.24) 38px 39px, transparent 39px 176px);
          border: 1px solid rgba(255,255,255,.30);
          box-shadow: inset 0 0 0 1px rgba(32,43,58,.018);
          opacity: .70;
          mask-image:
            radial-gradient(ellipse at 54% 50%, black 0%, black 56%, transparent 80%),
            linear-gradient(to bottom, transparent 0%, black 14%, black 84%, transparent 100%);
        }

        .operation-desktop-wallpaper::after {
          content: "";
          position: absolute;
          inset: 0;
          background-image:
            repeating-linear-gradient(90deg, transparent 0 30px, rgba(91,114,255,.050) 30px 31px, transparent 31px 160px),
            repeating-linear-gradient(0deg, transparent 0 46px, rgba(79,162,159,.044) 46px 47px, transparent 47px 184px),
            repeating-linear-gradient(135deg, rgba(255,255,255,.24) 0 1px, transparent 1px 34px),
            radial-gradient(rgba(18,28,42,.048) .65px, transparent .65px);
          background-size: 100% 100%, 100% 100%, 100% 100%, 24px 24px;
          opacity: .45;
          mask-image:
            linear-gradient(to bottom, transparent, black 12%, black 88%, transparent),
            linear-gradient(to right, transparent, black 10%, black 90%, transparent);
        }

        .operation-desktop-shadow {
          position: absolute;
          left: 8%;
          right: 8%;
          bottom: 48px;
          height: 32px;
          border-radius: 50%;
          background: rgba(66,80,102,.16);
          filter: blur(22px);
          pointer-events: none;
        }

        .operation-terminal-caret {
          display: inline-block;
          width: 7px;
          height: 14px;
          margin-left: 2px;
          background: #d9ffe5;
          animation: nexus-operation-caret 1s step-end infinite;
        }

        .operation-editor-caret {
          display: inline-block;
          width: 7px;
          height: 14px;
          margin-left: 2px;
          translate: 0 2px;
          background: #8de0ad;
          animation: nexus-operation-caret 1s step-end infinite;
        }

        .operation-editor-typed-line {
          display: inline-block;
          max-width: 100%;
          animation: nexus-operation-type-line 760ms steps(var(--operation-characters, 24), end) both;
          animation-delay: var(--operation-delay, 0ms);
        }

        .operation-editor-file-activity {
          animation: nexus-operation-editor-file-activity 1.65s ease-in-out infinite;
        }

        .operation-web-loading {
          position: relative;
          overflow: hidden;
        }

        .operation-web-loading::after {
          content: "";
          position: absolute;
          inset: 0;
          background: linear-gradient(110deg, transparent 0%, rgba(255,255,255,.18) 42%, transparent 62%);
          transform: translateX(-120%);
          animation: nexus-operation-shimmer 2.2s ease-in-out infinite;
        }

        .operation-diff-bar {
          height: 10px;
          border-radius: 999px;
          transform-origin: left center;
          animation: nexus-operation-pulse-width 1.8s ease-in-out infinite;
        }

        .operation-phase-meter {
          animation: nexus-operation-pulse-width 1.6s ease-in-out infinite;
          transform-origin: left center;
        }

        .operation-focus-dot {
          animation: nexus-operation-focus-dot 1.8s ease-in-out infinite;
        }

        .operation-stage-scene-enter {
          animation: nexus-operation-scene-enter 920ms cubic-bezier(.16,.84,.24,1) both;
        }

        .operation-idle-stage-exit {
          animation: nexus-operation-idle-exit 920ms cubic-bezier(.16,.84,.24,1) both;
          background: transparent !important;
        }

        .operation-idle-stage-exit .operation-idle-sky,
        .operation-idle-stage-exit .operation-idle-grid,
        .operation-idle-stage-exit .operation-idle-dotfield {
          opacity: 0;
          transition: opacity 180ms ease-out;
        }

        .operation-idle-stage-exit .operation-idle-agent-pill,
        .operation-idle-stage-exit .operation-idle-status-card,
        .operation-idle-stage-exit .operation-idle-clock {
          opacity: 0;
          transition: opacity 220ms ease-out;
        }

        .operation-boot-signal {
          animation: nexus-operation-boot-signal 1040ms cubic-bezier(.2,.8,.2,1) both;
        }

        .operation-boot-line {
          animation: nexus-operation-boot-line 1040ms cubic-bezier(.2,.8,.2,1) both;
          transform-origin: left center;
        }

        .operation-event-signal {
          animation: nexus-operation-event-signal 1400ms cubic-bezier(.16,.84,.24,1) both;
        }

        .operation-materializing-signal {
          animation: nexus-operation-materializing-signal 520ms cubic-bezier(.16,.84,.24,1) both;
        }

        .operation-materializing-line {
          animation: nexus-operation-materializing-line 980ms cubic-bezier(.2,.8,.2,1) both;
          transform-origin: left center;
        }

        @media (max-width: 767px) {
          .operation-stage-mobile-panel {
            left: auto !important;
            right: auto !important;
            width: 100% !important;
            min-width: 0 !important;
            max-width: 100% !important;
            transform: none !important;
          }
        }

        @media (prefers-reduced-motion: reduce) {
          .operation-stage-window,
          .operation-preview-line,
          .operation-editor-caret,
          .operation-editor-typed-line,
          .operation-editor-file-activity,
          .operation-scan-line,
          .operation-terminal-caret,
          .operation-web-loading::after,
          .operation-diff-bar,
          .operation-phase-meter,
          .operation-focus-dot,
          .operation-stage-scene-enter,
          .operation-idle-stage-exit,
          .operation-boot-signal,
          .operation-boot-line,
          .operation-event-signal,
          .operation-materializing-signal,
          .operation-materializing-line,
          .operation-window-traffic-icon {
            animation: none !important;
            transition: none !important;
          }
        }
      `}
    </style>
  );
}

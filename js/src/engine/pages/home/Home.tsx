import React from "react";
import PageComponent from "@engine/containers/PageComponent";

// The index page presents Vladimir Koroteev's CV inside a monitor that upgrades
// from a green CRT to a flat LCD as you scroll — the résumé scrolls through the
// glass while the hardware and typography modernize. One scroll gesture both
// reads the CV and "upgrades the machine", mirroring a 15-year career.

// Dark text sits on each monitor's own lit screen (which shows through the glass).
const S1_FG = "#0b140e"; // dark, near-black green on the pale-green CRT
const S2_FG = "#2e2408"; // dark brown on the amber CRT
const S3_FG = "#16181c"; // near-black on the white panel
// Page background = each image's measured edge colour, so the monitor blends
// into the page with no seam at any resolution.
const INSET1 = [13.9, 12.6, 12.6, 23.3];
const INSET2 = [10.6, 9.1, 8.1, 20.6];
const INSET3 = [10.4, 5.1, 5.1, 28.7];

// A short glitch burst fires when scroll progress crosses one of these — i.e.
// when the monitor switches over (green->amber at 1/3, amber->modern at 2/3).
const TRANSITIONS = [1 / 3, 2 / 3];

const clamp01 = (v: number) => Math.min(1, Math.max(0, v));

const toRGB = (h: string): [number, number, number] => {
  const n = parseInt(h.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
};
// Weighted blend of three colours.
const blend3 = (
  a: string, b: string, c: string,
  wa: number, wb: number, wc: number
): string => {
  const A = toRGB(a), B = toRGB(b), C = toRGB(c);
  const r = Math.round(A[0] * wa + B[0] * wb + C[0] * wc);
  const g = Math.round(A[1] * wa + B[1] * wb + C[1] * wc);
  const bl = Math.round(A[2] * wa + B[2] * wb + C[2] * wc);
  return `rgb(${r}, ${g}, ${bl})`;
};
const mix3 = (x: number, y: number, z: number, wa: number, wb: number, wc: number) =>
  x * wa + y * wb + z * wc;

interface Job {
  org: string;
  role: string;
  when: string;
  points: string[];
}

const EXPERIENCE: Job[] = [
  {
    org: "Semantic Lab, LLC",
    role: "Software Developer",
    when: "2024–2026",
    points: [
      "Built and owned a data ingestion pipeline that received, homogenized and stored operational data from a dental network expanding through acquisitions — adapters for heterogeneous sources, auth, the full path.",
      "Integrated several major LLM providers and shipped an interface for running language queries against the resulting data lake, from query to inference to results.",
    ],
  },
  {
    org: "Independent Consultant",
    role: "Software Developer",
    when: "2019–Present",
    points: [
      "Database query optimizations for MKRF and Forward.",
      "Legacy maintenance for Media Forensic and Forward: new modules, stack-change fixes, library upgrades.",
      "Front-end contract work for SupportLogic: interface updates and feature expansion.",
      "Network-topology optimization for a game engine, raising concurrent-client capacity (VK.com game platform).",
    ],
  },
  {
    org: "MKRF",
    role: "Software Engineer · Team Coordinator",
    when: "2018–2019",
    points: [
      "Led 6 front-end engineers building the web portal for a state-wide museum and art network; built a description-driven web-app framework in the spirit of Material.",
      "Partnered with backend to stand up a Tomcat backend (auth, DB, load-balancing) and standardized the front/back protocol for new use-cases.",
      "Championed design patterns, oversaw test coverage, and coached the team.",
    ],
  },
  {
    org: "Media Forensic",
    role: "Co-Founder · Lead Software Engineer",
    when: "2017–2019",
    points: [
      "Created a system that detects photo tampering for insurance-fraud investigation, backed by a database of digital markers for every major camera brand and model.",
      "Designed the C++/Python stack and led 4 engineers; delivered both a web client and an on-premise deployment.",
      "Worked directly with customers to deploy, adopt and adapt the product.",
    ],
  },
  {
    org: "Forward Exp",
    role: "Software Developer",
    when: "2012–2017",
    points: [
      "Built and maintained a logistics management system — shipments, clients, invoices, acts — with advanced filtering; later grown into a commercial product with a team.",
      "Created integrations with external systems: accounting software, shipping carriers, and more.",
      "Developed analytics reports, automation scripts and document-flow templates.",
      "Continuously profiled and improved performance: query optimization, network-load reduction.",
    ],
  },
];

const COMPETENCIES: Array<[string, string]> = [
  ["Languages & Frameworks", "Go, C++, Java, Swift, PHP, Python, JavaScript, TypeScript, Angular, React, React Native, Laravel, Django"],
  ["Data & Machine Learning", "R, Pandas, NumPy, Matplotlib, PyTorch, TensorFlow, Scikit-Learn, OpenCV"],
  ["Database & Infrastructure", "MySQL, PostgreSQL, MongoDB, Redis"],
  ["Tools & Methods", "Docker, Git, CI/CD, Agile/Scrum, architectural design, issue tracking"],
  ["Leadership", "Leading technical projects, coordinating cross-functional teams, mentoring developers"],
  ["Other", "MATLAB, Bash, VBasic, RESTful APIs, HTTP/2, SASS"],
];

interface IndexPageProps {}
interface IndexPageState {}

class Home extends PageComponent<IndexPageProps, IndexPageState> {
  protected title = "Home";
  protected href = "";
  protected isPage = false;

  private rootRef = React.createRef<HTMLDivElement>();
  private stageRef = React.createRef<HTMLDivElement>();
  private screenRef = React.createRef<HTMLDivElement>();
  private raf = 0;

  // Glitch intensity driven by scroll velocity + proximity to the mid-morph
  // "signal switch", with fast attack / slow release so it settles when idle.
  private glitch = 0;
  private glitchRaf = 0;
  private lastP = 0;

  async componentDidMount() {
    await super.componentDidMount();
    window.addEventListener("scroll", this.onScroll, { passive: true });
    window.addEventListener("resize", this.onScroll);
    this.update();
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.onScroll);
    window.removeEventListener("resize", this.onScroll);
    if (this.raf) cancelAnimationFrame(this.raf);
    if (this.glitchRaf) cancelAnimationFrame(this.glitchRaf);
  }

  private onScroll = () => {
    if (this.raf) return;
    this.raf = requestAnimationFrame(() => {
      this.raf = 0;
      this.update();
    });
  };

  private update = () => {
    const stage = this.stageRef.current;
    const screen = this.screenRef.current;
    const root = this.rootRef.current;
    if (!stage || !screen || !root) return;

    const total = stage.offsetHeight - window.innerHeight;
    const offset = -stage.getBoundingClientRect().top;
    const p = clamp01(offset / (total || 1));

    // Three overlapping crossfades: op1 (green) -> op2 (amber) -> op3 (modern),
    // crossovers centred on 1/3 and 2/3. Every property below is a weighted blend
    // of these, so colour, inset, glow and scanlines track the visible monitor.
    const t1 = 1 / 3, t2 = 2 / 3, w = 0.09;
    let op1 = clamp01((t1 + w - p) / (2 * w));
    let op3 = clamp01((p - (t2 - w)) / (2 * w));
    let op2 = clamp01(Math.min((p - (t1 - w)) / (2 * w), (t2 + w - p) / (2 * w)));
    const sum = op1 + op2 + op3 || 1;
    op1 /= sum; op2 /= sum; op3 /= sum;

    const s = root.style;
    s.setProperty("--op1", op1.toFixed(3));
    s.setProperty("--op2", op2.toFixed(3));
    s.setProperty("--op3", op3.toFixed(3));
    s.setProperty("--fg", blend3(S1_FG, S2_FG, S3_FG, op1, op2, op3));
    s.setProperty("--screen-top", mix3(INSET1[0], INSET2[0], INSET3[0], op1, op2, op3).toFixed(2) + "%");
    s.setProperty("--screen-left", mix3(INSET1[1], INSET2[1], INSET3[1], op1, op2, op3).toFixed(2) + "%");
    s.setProperty("--screen-right", mix3(INSET1[2], INSET2[2], INSET3[2], op1, op2, op3).toFixed(2) + "%");
    s.setProperty("--screen-bottom", mix3(INSET1[3], INSET2[3], INSET3[3], op1, op2, op3).toFixed(2) + "%");
    s.setProperty("--crt", (op1 + op2).toFixed(3));  // curvature: both CRTs, flat modern

    screen.scrollTop = p * (screen.scrollHeight - screen.clientHeight);
    root.classList.toggle("modern", op3 > 0.5);       // modern font on the flat panel

    // Glitch burst exactly at each switchover; decays near-instantly (glitchTick).
    for (const tp of TRANSITIONS) {
      if ((this.lastP - tp) * (p - tp) < 0) this.glitch = 1;
    }
    this.lastP = p;
    this.startGlitch();
  };

  private startGlitch = () => {
    if (!this.glitchRaf) this.glitchRaf = requestAnimationFrame(this.glitchTick);
  };

  private glitchTick = () => {
    const root = this.rootRef.current;
    this.glitch *= 0.4; // hard flash: gone in ~2-3 frames
    if (root) root.style.setProperty("--glitch", this.glitch.toFixed(3));

    if (this.glitch > 0.05) {
      this.glitchRaf = requestAnimationFrame(this.glitchTick);
    } else {
      this.glitch = 0;
      if (root) root.style.setProperty("--glitch", "0");
      this.glitchRaf = 0;
    }
  };

  render() {
    return (
      <div className="cv-root" ref={this.rootRef}>
        <div className="cv-stage" ref={this.stageRef}>
          <div className="cv-frame">
            <div className="cv-monitor">
              <img className="cv-frame-img s1" src="/img/crt-monitor.png" alt="" />
              <img className="cv-frame-img s2" src="/img/lcd-monitor.png" alt="" />
              <img className="cv-frame-img s3" src="/img/modern-monitor.png" alt="" />

              <div className="cv-screen" ref={this.screenRef}>
                <div className="cv-content">
                  <div className="cv-boot">
                    koroteev.dev — system ready<span className="cv-cursor" />
                  </div>

                  <h1 className="cv-name">Vladimir Koroteev</h1>
                  <p className="cv-role">
                    <b>Full-stack software engineer</b> · 15+ years · architectures, SaaS, applied math
                  </p>

                  <div className="cv-contact">
                    <a href="tel:+14085828466">(408) 582-8466</a>
                    <a href="mailto:vladimir@koroteev.dev">vladimir@koroteev.dev</a>
                    <a href="https://github.com/ws117z5">github.com/ws117z5</a>
                    <span className="cv-badge">Green Card holder</span>
                  </div>

                  <section className="cv-section">
                    <h2>Summary</h2>
                    <p>
                      Full-stack engineer who designs software architectures and builds web and SaaS
                      products end to end. Deep across front-end and back-end, and across storage —
                      relational, key-value, and document. I've built complete UI frameworks and design
                      languages, and my applied-math background lets me reach for custom protocols with
                      cryptographic primitives, neural networks, and backend network topologies when a
                      problem needs them. I like exploring new technology, guiding teams, and mentoring.
                    </p>
                  </section>

                  <section className="cv-section">
                    <h2>Experience</h2>
                    {EXPERIENCE.map((job) => (
                      <div className="cv-job" key={job.org}>
                        <div className="cv-job-head">
                          <span>
                            <span className="cv-job-org">{job.org}</span>{" "}
                            <span className="cv-job-role">— {job.role}</span>
                          </span>
                          <span className="cv-job-when">{job.when}</span>
                        </div>
                        <ul>
                          {job.points.map((pt, i) => (
                            <li key={i}>{pt}</li>
                          ))}
                        </ul>
                      </div>
                    ))}
                  </section>

                  <section className="cv-section">
                    <h2>Core competencies</h2>
                    <dl className="cv-grid">
                      {COMPETENCIES.map(([term, desc]) => (
                        <div key={term}>
                          <dt>{term}</dt>
                          <dd>{desc}</dd>
                        </div>
                      ))}
                    </dl>
                  </section>

                  <section className="cv-section">
                    <h2>Languages</h2>
                    <p>English — fluent, academic · Russian — native</p>
                  </section>
                </div>

                <div className="cv-static" />
                <div className="cv-flash" />
              </div>
            </div>
          </div>
        </div>

        <div className="cv-hint">scroll to upgrade ↓</div>
      </div>
    );
  }
}

export default Home;
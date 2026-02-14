# Stage 1: Enterprise Knowledge Platform

**Date:** 2025-11-18
**Status:** Go-to-Market Strategy (REVISED)
**Thesis:** Replace Confluence/Notion/SharePoint with living knowledge infrastructure, prove model with enterprise revenue, then expand to science and public internet

## Why Enterprise First?

### The Previous Plan (Science First)

**Challenges:**
- Long sales cycles (12-18 months for academic institutions)
- Need to prove value first (risk-averse)
- Grant-dependent (slow, uncertain funding)
- Cultural change required (researchers resistant to new tools)
- Chicken-and-egg (need content to be valuable, need users to get content)

**Time to first $10M:** 2-3 years

### The Better Plan (Enterprise First)

**Advantages:**
- Fast sales cycles (3-6 months)
- Clear, immediate pain (everyone hates current tools)
- Budget already allocated (replacing existing spend)
- Desperate for solutions (wasting millions on search time)
- Viral within orgs (one team → company-wide)

**Time to first $10M:** 12-18 months

**Then use enterprise revenue to fund science + public versions.**

## The Problem (Enterprise Knowledge Management is Broken)

### Current State of Enterprise Knowledge

**The Tool Sprawl:**

```
Typical Company Knowledge Stack:

Confluence/SharePoint
├── 10,000+ pages
├── 70% outdated (no one maintains)
├── Search doesn't work
├── Nobody knows what's current
└── "Just ask Steve"

Google Docs / Notion
├── Scattered across drives
├── Duplicated information
├── Version chaos ("final_v2_FINAL.docx")
├── Permission mess
└── Lost when someone leaves

Slack / Email
├── Knowledge buried in threads
├── Impossible to find later
├── Context lost
├── Disappears after 90 days (free tier)
└── Institutional memory in individual inboxes

Code / GitHub
├── READMEs out of date
├── Comments contradict code
├── Architecture undocumented
├── "Why did we do this?" (git blame: developer left 3 years ago)
└── Tribal knowledge

People's Heads
├── Ask Steve (he's been here 10 years)
├── Steve leaves → knowledge gone
├── Onboarding: "Shadow someone for 3 months"
├── Single points of failure
└── Not scalable
```

**The Costs:**

**Time Waste:**
- IDC study: Knowledge workers spend **2.5 hours/day** searching for information
- 1000 employees × 2.5 hours × $50/hour × 250 days/year = **$31.25M/year wasted**

**Knowledge Loss:**
- Employee leaves → knowledge leaves
- Gartner: Fortune 500 companies lose **$31.5B/year** from failure to share knowledge

**Onboarding Pain:**
- New employee productivity: 3-6 months to "get up to speed"
- Why? Can't find information, tribal knowledge, contradictory docs

**Decision-Making:**
- "We tried that in 2019, didn't work"
- "Did we? I don't remember"
- No institutional memory → repeat failures

**Compliance/Legal:**
- Can't prove what employees knew/when
- Compliance documentation scattered
- Audit nightmare

### What Companies Really Want

**When you ask knowledge workers:**

"I just want to:"
- ✅ Ask a question, get an answer (with source)
- ✅ Know what's actually true (not 5 contradictory docs)
- ✅ See who knows what (find the expert)
- ✅ Understand why decisions were made (context, not just outcome)
- ✅ Have docs that stay current (not instantly stale)
- ✅ Onboard in days, not months (knowledge accessible)

**When you ask executives:**

"I want to:"
- ✅ Not lose knowledge when people leave
- ✅ Speed up decision-making (everyone has context)
- ✅ Reduce onboarding time (faster productivity)
- ✅ AI that knows our company (trained on our knowledge)
- ✅ ROI on knowledge tools (currently negative)

**Nobody is happy with current solutions.**

## The Solution: noldarim Enterprise Knowledge Platform

### What It Is

**"Living knowledge infrastructure for enterprises"**

Think:
- Wikipedia (collaborative, versioned)
- + GitHub (pull requests for knowledge, version control)
- + Notion (beautiful, fast)
- + ChatGPT (AI that answers questions)
- **But actually designed for internal company knowledge**

### Core Features

#### 1. Knowledge Graph (Not Documents)

**Current tools:** Documents, wikis, pages

**noldarim:** Entities + Relationships + Evidence

```
Example: Product Launch Process

Old way (Confluence doc):
---
# Product Launch Checklist

1. Get legal approval
2. Update marketing site
3. Train sales team
4. ...

(Document written 2 years ago, half the steps outdated)
---

New way (noldarim Knowledge Graph):

Entity: ProductLaunchProcess
├── Steps:
│   ├── LegalApproval
│   │   ├── Owner: Legal team
│   │   ├── Typically: 2 weeks
│   │   ├── Blocker: Often data privacy review
│   │   ├── Contact: Sarah (legal)
│   │   └── Updated: 2 days ago (Sarah updated)
│   │
│   ├── MarketingSiteUpdate
│   │   ├── Owner: Marketing
│   │   ├── Typically: 1 week
│   │   ├── Depends: Legal approval (content claims)
│   │   ├── Contact: James (marketing)
│   │   └── Updated: 1 week ago (James)
│   │
│   └── SalesTraining
│       ├── Owner: Sales enablement
│       ├── Typically: 3 days
│       ├── Format: Video + live Q&A
│       ├── Contact: Lisa (enablement)
│       └── Updated: Yesterday (Lisa)
│
├── Recent Launches:
│   ├── Product X (Jan 2025): 6 weeks (legal delayed)
│   ├── Product Y (Dec 2024): 4 weeks (smooth)
│   └── Product Z (Nov 2024): 8 weeks (marketing redesign)
│
├── Common Issues:
│   ├── Legal approval takes longer than expected (70% of launches)
│   ├── Sales training needs more notice (feedback from 3 launches)
│   └── Marketing site updates block on legal (dependency issue)
│
└── Who Knows This:
    ├── Sarah (legal) - 12 launches
    ├── James (marketing) - 8 launches
    ├── Lisa (enablement) - 15 launches
    └── Expert: Lisa (most experience)

Status: CURRENT (updated by owners in last 7 days)
Confidence: HIGH (3 active owners maintaining)
```

**Key differences:**
- ✅ Structured (queryable, not just searchable)
- ✅ Living (owners update, not stale docs)
- ✅ Connected (relationships explicit)
- ✅ Versioned (see how it evolved)
- ✅ Attributed (know who to ask)
- ✅ Current (freshness tracked)

#### 2. Living Documentation (Auto-Updates)

**The problem:** Docs go stale the moment they're written

**The solution:** Docs that update themselves

```
Example: API Documentation

Traditional (static):
---
# Users API

GET /api/users/{id}

Returns user data.

(Written 2 years ago, endpoint changed 6 times since)
---

noldarim (living):

Entity: UsersAPI_GetUser
├── Endpoint: GET /api/users/{id}
│   └── Source: Code (extracted from OpenAPI spec)
│       └── Last changed: 3 days ago (commit abc123)
│
├── Purpose: "Fetch user profile data"
│   └── Source: PR #1234 description
│
├── Authentication: Required (OAuth2)
│   └── Source: Auth middleware (code)
│
├── Response Schema:
│   ├── id: string
│   ├── name: string
│   ├── email: string
│   ├── created_at: timestamp
│   └── Source: TypeScript interface (code)
│       └── Auto-synced from codebase
│
├── Performance:
│   ├── p95 latency: 45ms
│   ├── Error rate: 0.1%
│   └── Source: Production metrics (Datadog)
│       └── Updated: Real-time
│
├── Common Issues:
│   ├── "Returns 403 if user is deleted" (learned from 3 support tickets)
│   ├── "Slow if user has 1000+ friends" (incident #456)
│   └── Source: Support tickets + incidents
│
├── Recent Changes:
│   ├── Added created_at field (v2.3, 3 days ago)
│   ├── Removed deprecated status field (v2.1, 1 month ago)
│   └── Source: Git history
│
└── Who Maintains:
    ├── Team: Backend Engineering
    ├── Primary: Alice (wrote initial version)
    ├── Recent: Bob (added created_at)
    └── Source: Git commits + team ownership

Status: CURRENT (synced from code 1 hour ago)
```

**How it works:**
- Watch codebase (git commits, PRs)
- Extract from code (OpenAPI specs, TypeScript types, comments)
- Monitor production (metrics, errors, incidents)
- Learn from support (tickets, issues, questions)
- Combine into living understanding

**Result:** Docs never stale

#### 3. AI That Actually Knows Your Company

**Current "ChatGPT for our company" solutions:**

```
Problem: "How do I deploy to production?"

RAG over Confluence:
├── Searches confluence for "deploy production"
├── Finds 47 docs (half outdated)
├── Returns chunks of text
├── Contradictions everywhere
├── AI hallucinates to fill gaps
└── Answer: Wrong

User: "This doesn't work"
Support: "Oh that doc is from 2020, we do it differently now"
```

**noldarim AI:**

```
User: "How do I deploy to production?"

AI:
1. Queries knowledge graph (not text search)
   → Find entity: ProductionDeploymentProcess

2. Gets structured answer:
   ├── Current process (updated 2 days ago)
   ├── Steps with owners
   ├── Common pitfalls (learned from incidents)
   ├── Who to ask if blocked (expert: DevOps team)
   └── Recent changes (what changed, why)

3. Generates answer with sources:

"To deploy to production:

1. Create PR, get 2 approvals
   (Source: DeploymentPolicy, updated yesterday by Alice)

2. Merge to main
   (Triggers CI/CD automatically)

3. CI runs tests, builds Docker image
   (Usually takes 10 minutes)
   (Source: CI/CD pipeline config)

4. Auto-deploys to staging
   (Waits 30 minutes for smoke tests)

5. Approval needed for production
   (Slack #deployments channel)
   (DevOps team approves, typically < 1 hour)

6. Auto-deploys to production
   (Rolling deployment, 5 minutes)

Common issues:
- If tests fail: Check #engineering-support
- If approval delayed: Ping @devops in Slack
- If deployment fails: Automatic rollback, check logs

Last changed: 2 days ago (added smoke test wait)
Reason: "Prevent bad deploys" (incident #789 post-mortem)

Need help? Ask Alice (DevOps) or Bob (recent deployer)"

Confidence: HIGH (current, verified by 5 recent deploys)
Sources: [Links to policies, code, incidents]
```

**Why this works:**
- ✅ Structured knowledge (not text chunks)
- ✅ Current (synced from reality)
- ✅ Verified (multi-source confirmation)
- ✅ Attributed (know who to ask)
- ✅ Explainable (cite sources)
- ✅ No hallucination (only what's in graph)

#### 4. Expertise Discovery

**Problem:** "Who knows about X?"

**Current solution:** Ask around, hope someone knows

**noldarim:**

```
Query: "Who knows about our payment processing system?"

Results (ranked by actual expertise):

1. Sarah Chen (Backend Engineering)
   ├── Wrote initial implementation (2019)
   ├── Maintained for 3 years
   ├── Resolved 47 payment incidents
   ├── Contributed 156 docs/updates
   ├── Last active: Yesterday (updated docs)
   └── Expertise score: 98/100

2. James Liu (Staff Engineer)
   ├── Refactored payment flow (2022)
   ├── Resolved 23 payment incidents
   ├── Wrote fraud detection system
   ├── Contributed 89 docs/updates
   ├── Last active: 1 week ago
   └── Expertise score: 87/100

3. Current team: Payments squad
   ├── Members: 5 engineers
   ├── Collective knowledge
   ├── Active: Daily updates
   └── Expertise score: 82/100

[Contact] [See their contributions] [View knowledge areas]
```

**How expertise is calculated:**
- Code contributions (commits, PRs)
- Documentation (created, updated)
- Incident resolution (fixed issues)
- Recency (still active?)
- Peer verification (others cite them)

**No self-reporting. Objective, data-driven.**

#### 5. Version Control for Knowledge

**Like git, but for knowledge:**

```
Product Launch Process (entity)

History:
├── v1.0 (Jan 2023): Initial process (Sarah)
│   └── 5 steps, simple
│
├── v1.1 (Mar 2023): Added legal review (Lisa)
│   └── Reason: "Compliance requirement"
│
├── v2.0 (Jun 2023): Restructured (James)
│   └── Reason: "Previous process caused delays"
│   └── Changes: Parallelized legal + marketing
│
├── v2.1 (Sep 2023): Added sales training (Alice)
│   └── Reason: "Sales team complained about lack of notice"
│
├── v2.2 (Jan 2024): Updated legal timeline (Sarah)
│   └── Reason: "Legal team hired more people, faster now"
│
└── v3.0 (Current): Major revision (Bob)
    └── Reason: "Learned from 15 launches, streamlined"
    └── Impact: Reduced launch time from 8 weeks → 4 weeks

[View any version] [See diff] [Fork for experiment]
```

**Benefits:**
- See how knowledge evolved
- Understand why decisions made
- Learn from history (don't repeat failures)
- Rollback if needed (new process worse? revert)
- Fork for experiments (try new approach without breaking current)

#### 6. Integration with Existing Tools

**Not rip-and-replace. Augment.**

```
Integrations:

Code:
├── GitHub (sync code, PRs, issues)
├── GitLab
└── Bitbucket

Docs:
├── Confluence (import existing, keep in sync)
├── Notion (bi-directional sync)
└── Google Docs (import, track changes)

Communication:
├── Slack (answer questions inline, capture knowledge)
├── Teams
└── Email (extract decisions from threads)

Monitoring:
├── Datadog (metrics, incidents)
├── Sentry (errors)
├── PagerDuty (incidents)
└── New Relic

Project Management:
├── Jira (link knowledge to tickets)
├── Linear
└── Asana

Dev Tools:
├── CI/CD (GitHub Actions, Jenkins)
├── Cloud (AWS, GCP, Azure - infra as knowledge)
└── Databases (schema as knowledge)
```

**Workflow:**
1. Import existing knowledge (Confluence, Notion, Docs)
2. Connect to tools (GitHub, Slack, monitoring)
3. Knowledge auto-updates from integrations
4. Users ask questions in Slack → AI answers with sources
5. Gradually migrate from old tools to noldarim

**No "big bang" migration. Incremental adoption.**

## Go-to-Market Strategy

### Target Market

**Primary:** Mid-market to enterprise tech companies (500-5000 employees)

**Why:**
- ✅ Clear pain (knowledge chaos)
- ✅ Budget allocated (paying for Confluence, Notion, etc.)
- ✅ Technical sophistication (understand value)
- ✅ Fast decision-making (not enterprise bureaucracy)

**Ideal Customer Profile:**

```
Company:
├── Size: 500-5000 employees
├── Type: SaaS, fintech, healthtech, etc.
├── Growth stage: Series B-D (scaling, hiring fast)
├── Pain: Knowledge scattered, onboarding slow
└── Budget: $500K-5M/year on knowledge tools

Buyer:
├── Title: VP Engineering, CTO, Head of Ops
├── Pain: Team can't find information, wasting time
├── Metric: Wants to reduce onboarding time, increase productivity
└── Budget: Has $50-200/employee/month for tools
```

**Expansion:** After proving in tech, expand to finance, healthcare, consulting, legal

### Pricing

**Competitive Analysis:**

```
Current spend (1000 employees):

Confluence:
- $5-10/user/month = $5K-10K/month
- Enterprise: $10-20/user/month

Notion:
- $8-15/user/month = $8K-15K/month
- Enterprise: $15-25/user/month

Total: $15-35K/month = $180K-420K/year

Plus:
- SharePoint/OneDrive (Microsoft 365)
- Slack paid tier
- GitHub Enterprise
- Monitoring tools

Actual knowledge stack: $500K-1M/year
```

**noldarim Pricing:**

```
Starter: $25/user/month
├── Knowledge graph
├── Living docs (basic)
├── AI Q&A (100 queries/user/month)
├── Integrations (GitHub, Slack)
└── Up to 100 users

Professional: $50/user/month
├── Everything in Starter
├── Unlimited AI queries
├── Advanced integrations (monitoring, etc.)
├── Custom workflows
├── Priority support
└── Up to 1000 users

Enterprise: $75-150/user/month (volume discounts)
├── Everything in Professional
├── Custom AI training (on your knowledge)
├── Dedicated support
├── SLA guarantees
├── Security/compliance (SOC2, HIPAA)
├── On-prem deployment option
└── 1000+ users
```

**Example customer (1000 employees):**

```
Professional tier:
- $50/user × 1000 users = $50K/month = $600K/year

Value prop:
- Replace Confluence ($120K) + Notion ($180K) = $300K savings? No.
- Actually: Pay MORE, get 10x value

ROI calculation:
- Time saved: 2.5 hrs/day → 1 hr/day (AI answers instantly)
- 1.5 hrs × 1000 employees × $50/hr × 250 days = $18.75M/year saved
- Cost: $600K/year
- ROI: 31x

Sell on value, not cost.
```

### Sales Strategy

**Phase 1: Founder-Led Sales (Months 0-6)**

**Goal:** 5 customers, prove value

**Approach:**
1. Find 10 friendly companies (warm intros, existing network)
2. Offer pilot (50% discount, 3 months)
3. Deploy with one team (engineering, product)
4. Measure impact (time saved, questions answered, satisfaction)
5. Expand to company-wide
6. Get testimonial + case study

**Metrics to prove:**
- Time to find information: Before vs After
- Onboarding time: Before vs After
- AI accuracy: % questions answered correctly
- User satisfaction: NPS score

**Target:** $1M ARR (5 customers × $200K average)

**Phase 2: Sales Team (Months 6-18)**

**Goal:** 50 customers, scale revenue

**Approach:**
1. Hire 3-5 AEs (Account Executives)
2. Hire 1 SDR team lead + 3 SDRs
3. Build sales playbook (from founder learnings)
4. Target: Tech companies 500-5K employees
5. Channels:
   - Outbound (SDRs)
   - Inbound (content marketing, SEO)
   - Partnerships (consultancies, VCs)
   - Events (tech conferences)

**Metrics:**
- Pipeline: $20M
- Close rate: 25%
- Average deal: $400K
- Sales cycle: 4 months

**Target:** $20M ARR (50 customers × $400K average)

**Phase 3: Scale (Months 18-36)**

**Goal:** 200 customers, category leader

**Approach:**
1. Sales team: 20+ AEs, 10+ SDRs
2. Channels:
   - Enterprise sales (1000+ employees)
   - Product-led growth (free tier → paid)
   - Partnerships (consultancies deploy for clients)
   - Integrations (marketplace, Slack app store)
3. International expansion (EU, Asia)
4. Vertical expansion (finance, healthcare, legal)

**Target:** $100M ARR (200 customers × $500K average)

### Product-Led Growth (PLG) Strategy

**Not just enterprise sales. Viral growth too.**

**Free tier (for individuals/small teams):**

```
Free:
├── 10 users
├── 1000 entities
├── 100 AI queries/month
├── Basic integrations
└── Community support

Upgrade path:
- Team grows → needs more users
- Knowledge grows → needs more entities
- AI usage grows → needs more queries
- Features needed → upgrades to paid

Then:
- Team loves it
- Brings to company leadership
- "Can we get this for whole company?"
- Becomes enterprise deal
```

**Like Slack, like Notion—viral within orgs, bottom-up adoption.**

## Revenue Model

### Year 1: $12M ARR

**Customer acquisition:**

```
Q1: 2 customers (pilot)
├── $150K average
└── $300K ARR

Q2: 5 customers
├── $200K average
└── $1M ARR

Q3: 15 customers (sales team ramping)
├── $250K average
└── $3.75M ARR

Q4: 30 customers
├── $300K average
└── $9M ARR

End of Year 1: 50 customers, $12M ARR
```

**Customer breakdown:**
- Avg deal size: $250K
- Avg company size: 800 employees
- Price: ~$30/user/month (volume discounts)

### Year 2: $50M ARR

**Growth:**
- Sales team: 15 AEs (ramped)
- Each AE: $2M quota
- Attainment: 80%
- Total new ARR: $24M

**Existing customers:**
- Retention: 95% (very high, sticky product)
- Expansion: 30% (more users, more features)
- Base: $12M × 1.25 (retention + expansion) = $15M

**Inbound/PLG:**
- Product-led: $5M
- Partnerships: $3M

**Total Year 2: $47M ARR** (round to $50M)

**Customer count:** ~150 customers

### Year 3: $150M ARR

**Continued growth:**
- Sales team: 40 AEs
- Quota: $2.5M each
- Attainment: 75%
- New ARR: $75M

**Existing base:**
- $50M × 1.2 = $60M

**PLG + partnerships:**
- $15M

**Total Year 3: $150M ARR**

**Customer count:** 400-500 customers

### Year 5: $500M ARR

**At scale:**
- 2000+ enterprise customers
- PLG: Tens of thousands of small teams
- International: 40% revenue
- Verticals: Tech (50%), Finance (20%), Healthcare (15%), Other (15%)

## AI Training Revenue (Additional)

**Dual monetization:**

### 1. Customer AI (Train on their knowledge)

**Every enterprise customer wants:**
"ChatGPT trained on OUR company knowledge"

**Our offering:**
```
AI Add-on: $25-50/user/month additional

Features:
├── Custom model (trained on company knowledge graph)
├── Accurate answers (no hallucination)
├── Source attribution (cites internal docs)
├── Always current (retrains as knowledge updates)
└── Privacy guaranteed (model stays with company)

Adoption:
- 50% of customers add AI tier
- Average: $35/user/month additional

Revenue impact:
Year 1: $12M base → $3M AI add-on = $15M total
Year 3: $150M base → $50M AI add-on = $200M total
Year 5: $500M base → $200M AI add-on = $700M total
```

### 2. General AI (Public Knowledge Internet)

**Later stage (Year 3+):**

Once we have public version (knowledge internet):
- Aggregate public knowledge (science, medicine, law)
- Train foundation models
- Sell to AI companies (OpenAI, Anthropic, Google)

**Revenue:** $50-200M/year (as described in knowledge_internet.md)

## Why This Works

### 1. Clear Pain

Everyone knows enterprise knowledge is broken. No need to convince.

### 2. Fast ROI

```
Customer: 1000 employees

Time saved:
- 2.5 hrs/day searching → 1 hr/day
- 1.5 hrs × 1000 employees × 250 days = 375K hours
- × $50/hour = $18.75M value

Cost: $600K/year

ROI: 31x

Payback period: 2 weeks
```

**This is a no-brainer purchase.**

### 3. Viral Adoption

Start with one team → spreads organically → company-wide adoption

### 4. Network Effects

More usage → more knowledge → better AI → more valuable → more usage

### 5. High Switching Costs (Good Kind)

Once knowledge is in noldarim:
- Hard to migrate out (but we allow export)
- AI trained on it (valuable)
- Workflows built on it (integrated)
- Team relies on it (critical path)

**Stay because it's valuable, not because locked in.**

### 6. Expansion Revenue

```
Initial sale: 100 users, $5K/month

6 months later:
- Expanded to 500 users ($25K/month)
- Added AI tier ($12.5K/month)
- Total: $37.5K/month (7.5x expansion)

Net revenue retention: 150%+ (very high)
```

### 7. Category Creation

We're not "better Confluence"—we're a **new category**:

**"Living Knowledge Infrastructure"**

- Knowledge graphs (not docs)
- AI-native (not search)
- Self-updating (not static)
- Provenance-tracked (not trust-based)

**Category leadership → premium pricing → market dominance**

## Competitive Landscape

### Direct Competitors (They're not really competitors)

**Confluence (Atlassian):**
- Strengths: Market leader, integrated with Jira
- Weaknesses: Static docs, bad search, goes stale
- Our advantage: Living knowledge, AI, actually works

**Notion:**
- Strengths: Beautiful UX, flexible
- Weaknesses: Still documents, no AI (yet), no provenance
- Our advantage: Knowledge graph, enterprise AI, auto-updates

**SharePoint:**
- Strengths: Enterprise, Microsoft integration
- Weaknesses: Everyone hates it, unusable
- Our advantage: Everything

### Emerging Competitors

**Glean (enterprise search):**
- Strengths: Good search across tools
- Weaknesses: Search, not knowledge. Doesn't solve fragmentation
- Our advantage: Knowledge graph, not just better search

**Guru / Tettra (knowledge management):**
- Strengths: Q&A, verification
- Weaknesses: Still cards/docs, manual verification
- Our advantage: Automated, living, AI-native

**Document.ai / Dashworks (AI search):**
- Strengths: AI-powered search
- Weaknesses: RAG over messy docs (garbage in, garbage out)
- Our advantage: Structured knowledge, not text chunks

### Why We Win

**They're all building on top of broken foundations (documents).**

**We're building the right foundation (knowledge graphs).**

Once customers see the difference:
- Structured vs unstructured
- Living vs static
- AI that works vs AI that hallucinates
- Provenance vs trust-me

**No going back.**

## Customer Success Metrics

**What we optimize for:**

### User Metrics
- Time to find information: <30 seconds (from 30 minutes)
- AI answer accuracy: >95%
- Questions answered by AI: >80% (reduce human ask-around)
- User satisfaction: NPS >50

### Business Metrics
- Onboarding time: 1 week (from 3 months)
- Employee productivity: +20% (measured via survey)
- Knowledge retention: 95% (when employee leaves, knowledge stays)
- Decision-making speed: +50% (context readily available)

### Technical Metrics
- Knowledge freshness: <7 days avg (auto-updates)
- Graph completeness: >80% of company knowledge captured
- Integration coverage: All major tools connected
- Uptime: 99.9%+

## Why This is the Right Stage 1

**Compared to Science:**

| | Enterprise | Science |
|---|-----------|---------|
| **Sales cycle** | 3-6 months | 12-18 months |
| **Customer budget** | $500K-1M already allocated | Grant-dependent |
| **Pain awareness** | Obvious, immediate | Have to educate |
| **Decision makers** | VPs, CTOs (fast) | Committees (slow) |
| **Time to $10M** | 12-18 months | 2-3 years |
| **Viral potential** | High (within orgs) | Low (institutional) |

**Then use enterprise revenue to fund science + public versions.**

**Virtuous cycle:**
```
Enterprise revenue
    ↓
Fund science platform (free for academics)
    ↓
Academics create high-quality knowledge
    ↓
Public knowledge internet
    ↓
Better AI training data
    ↓
More revenue from AI companies
    ↓
Fund more innovation
```

---

## Next Steps

### Month 1: Validate

**Talk to 20 potential customers:**
- VPs of Engineering
- CTOs
- Heads of Operations

**Questions:**
- How do you manage internal knowledge today?
- What's most painful?
- What have you tried?
- What would you pay for solution?

**Goal:** Confirm pain, validate pricing, find pilot customers

### Month 2-3: Build MVP

**Minimum viable product:**
- Knowledge graph (core)
- Confluence import
- GitHub integration
- AI Q&A (basic)
- Web interface

**NOT building:**
- All integrations (just 2-3)
- Advanced AI (basic RAG is fine for MVP)
- Mobile apps
- Advanced workflows

**Goal:** Something usable for pilot

### Month 4-6: Pilot with 3 Customers

**Deploy:**
- 3 friendly companies
- 50-100 employees each
- One team initially

**Measure:**
- Time to find info (before/after)
- AI accuracy
- User satisfaction
- Expansion potential

**Goal:** Proof of value, testimonials, case studies

### Month 7-12: First $1M ARR

**Scale:**
- Hire 2 AEs
- Iterate on product (pilot learnings)
- 5-10 customers
- $100-200K average deal

**Goal:** Repeatable sales motion, product-market fit

### Month 13-24: $10-20M ARR

**Build sales machine:**
- 10+ AEs
- Marketing team
- 50-100 customers
- Category creation

**Goal:** Clear market leader, path to $100M

---

## The Pitch (60 seconds)

**"Enterprise knowledge management is broken. Your employees waste 2.5 hours a day searching for information. Knowledge lives in Confluence, Notion, Google Docs, Slack, people's heads—contradictory, outdated, lost when people leave.**

**We're building living knowledge infrastructure—not documents, but knowledge graphs that auto-update from your code, docs, and tools. AI that actually knows your company, trained on structured, verified knowledge.**

**Replace Confluence, Notion, tribal knowledge. ROI: 31x. Payback: 2 weeks.**

**Starting with tech companies (500-5K employees). $50/user/month. Proven with 5 pilots, 95% reduction in search time.**

**Year 1 target: $12M ARR. Then use revenue to build science platform and public knowledge internet.**

**Built on noldarim—proven workflow infrastructure. Foundation-owned IP, mission-protected.**

**Let's fix enterprise knowledge. Then fix all knowledge."**

---

**This is it. This is the wedge.**

Enterprise first → Science second → Public knowledge internet third → AI training revenue throughout.

**Fast to revenue. High margins. Massive market. Clear path to $500M+.**

**Let's build this.**

import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';

import styles from './index.module.css';

function BadgeBar() {
  return (
    <div className={styles.badgeBar} role="group" aria-label="Project badges">
      <a
        href="https://github.com/feather-store/feather"
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Star Feather on GitHub">
        <img
          src="https://img.shields.io/github/stars/feather-store/feather?style=social"
          alt="GitHub Stars"
          aria-hidden="true"
        />
      </a>
      <img
        src="https://img.shields.io/badge/go-%3E%3D1.22-00ADD8?logo=go&logoColor=white"
        alt="Requires Go version 1.22 or higher"
        aria-label="Go version requirement: 1.22 or higher"
      />
      <img
        src="https://img.shields.io/badge/license-MIT-green"
        alt="MIT License"
        aria-label="Licensed under MIT"
      />
      <img
        src="https://img.shields.io/badge/PRs-welcome-brightgreen"
        alt="PRs Welcome"
        aria-label="Pull requests are welcome"
      />
    </div>
  );
}

type BuiltForItem = {
  icon: string;
  title: string;
  desc: string;
};

const builtForItems: BuiltForItem[] = [
  { icon: '🤖', title: 'ML Engineers', desc: 'Real-time inference' },
  { icon: '📊', title: 'Data Scientists', desc: 'Training datasets' },
  { icon: '⚙️', title: 'Platform Teams', desc: 'Easy to operate' },
  { icon: '🚀', title: 'Startups', desc: 'Ship faster' },
];

function BuiltForSection() {
  return (
    <section className={styles.builtForSection}>
      <div className="container">
        <div className={styles.builtForGrid}>
          {builtForItems.map((item, idx) => (
            <div key={idx} className={styles.builtForItem}>
              <div className={styles.builtForIcon}>{item.icon}</div>
              <div className={styles.builtForTitle}>{item.title}</div>
              <div className={styles.builtForDesc}>{item.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <img
          src="/img/logo.svg"
          alt="Feather Logo"
          className={styles.heroLogo}
        />
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <p className={styles.heroTagline}>
          Sub-millisecond P99 latency • Single binary deployment • No external dependencies
        </p>
        <BadgeBar />
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started">
            Get Started
          </Link>
          <Link
            className="button button--outline button--lg"
            to="https://github.com/feather-store/feather"
            style={{marginLeft: '1rem', color: 'white', borderColor: 'white'}}>
            View on GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

function InstallSection() {
  return (
    <section className={styles.installSection}>
      <div className="container">
        <div className="row">
          <div className="col col--6">
            <Heading as="h2">Install in Seconds</Heading>
            <p>
              Download the binary and run. No Docker required, no external
              databases, no complex setup. Just a single executable.
            </p>
            <CodeBlock language="bash" title="Quick Install">
{`# Download and run
curl -sSL https://github.com/feather-store/feather/releases/latest/download/feather-linux-amd64 -o feather
chmod +x feather
./feather

# Or with Docker
docker run -p 8080:8080 -p 50051:50051 ghcr.io/feather-store/feather:latest`}
            </CodeBlock>
          </div>
          <div className="col col--6">
            <Heading as="h2">Serve Features Instantly</Heading>
            <p>
              Store and retrieve ML features with a simple HTTP API.
              Sub-millisecond latency out of the box.
            </p>
            <CodeBlock language="bash" title="Store and Retrieve">
{`# Store features
curl -X POST http://localhost:8080/v1/features \\
  -H "Content-Type: application/json" \\
  -d '{"entity_key": "user:123", "features": {"clicks": 42}}'

# Retrieve features
curl "http://localhost:8080/v1/features?entity=user:123&feature=clicks"
# {"data":{"entities":{"user:123":{"features":{"clicks":{"value":42}}}}}}`}
            </CodeBlock>
          </div>
        </div>
      </div>
    </section>
  );
}

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Sub-Millisecond Latency',
    icon: '⚡',
    description: (
      <>
        P99 latency under 1ms for hot tier reads. 256-shard in-memory cache
        with fine-grained locking serves 1M+ ops/sec on a single node.
      </>
    ),
  },
  {
    title: 'Single Binary Deployment',
    icon: '📦',
    description: (
      <>
        No Redis, no PostgreSQL, no external dependencies. Embedded BadgerDB
        for persistence. Download and run in under 5 minutes.
      </>
    ),
  },
  {
    title: 'Point-in-Time Queries',
    icon: '⏱️',
    description: (
      <>
        Retrieve features as they existed at any timestamp. Generate
        training data without data leakage. Essential for ML reproducibility.
      </>
    ),
  },
  {
    title: 'Vector Similarity Search',
    icon: '🔍',
    description: (
      <>
        Built-in HNSW index for embedding search. Cosine, Euclidean, and
        dot product distances. No separate vector database needed.
      </>
    ),
  },
  {
    title: 'Real-Time Aggregations',
    icon: '📊',
    description: (
      <>
        Sliding window aggregations (count, sum, avg, min, max) computed
        incrementally. Define windows from minutes to days.
      </>
    ),
  },
  {
    title: 'Production Ready',
    icon: '🛡️',
    description: (
      <>
        Prometheus metrics, OpenTelemetry tracing, structured logging,
        health probes. Built for Kubernetes from day one.
      </>
    ),
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4', styles.featureCol)}>
      <div className={styles.featureCard}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

function FeaturesSection() {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="text--center margin-bottom--xl">
          <Heading as="h2">Why Feather?</Heading>
          <p className={styles.featuresSubtitle}>
            Built for ML teams who need fast, reliable feature serving without operational complexity
          </p>
        </div>
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

function PerformanceSection() {
  return (
    <section className={styles.performanceSection}>
      <div className="container">
        <div className="text--center margin-bottom--lg">
          <Heading as="h2">Performance at Scale</Heading>
          <p>Benchmarked on AWS c5.4xlarge (16 vCPU, 32GB RAM)</p>
        </div>
        <div className={styles.statsGrid}>
          <div className={styles.statCard}>
            <div className={styles.statNumber}>{'<1ms'}</div>
            <div className={styles.statLabel}>P99 Hot Tier Latency</div>
          </div>
          <div className={styles.statCard}>
            <div className={styles.statNumber}>1M+</div>
            <div className={styles.statLabel}>Ops/Second</div>
          </div>
          <div className={styles.statCard}>
            <div className={styles.statNumber}>1B</div>
            <div className={styles.statLabel}>Features in 64GB RAM</div>
          </div>
          <div className={styles.statCard}>
            <div className={styles.statNumber}>5ms</div>
            <div className={styles.statLabel}>Vector Search (1M vectors)</div>
          </div>
        </div>
      </div>
    </section>
  );
}

function ArchitectureSection() {
  return (
    <section className={styles.architectureSection}>
      <div className="container">
        <div className="row">
          <div className="col col--6">
            <Heading as="h2">Two-Tier Architecture</Heading>
            <p>
              Feather uses a tiered storage architecture to optimize for both
              latency and durability:
            </p>
            <ul>
              <li>
                <strong>Hot Tier:</strong> In-memory LRU cache with 256 shards.
                Sub-millisecond reads, automatic eviction.
              </li>
              <li>
                <strong>Warm Tier:</strong> BadgerDB-backed persistent storage.
                Historical versions for point-in-time queries.
              </li>
            </ul>
            <p>
              Reads check the hot tier first. Cache misses fall through to warm
              tier. Writes update hot tier synchronously and warm tier asynchronously.
            </p>
            <Link
              className="button button--primary"
              to="/docs/concepts/architecture">
              Learn More
            </Link>
          </div>
          <div className="col col--6">
            <div className={styles.architectureDiagram}>
              <img
                src="/img/architecture-diagram.svg"
                alt="Feather two-tier architecture diagram showing client requests flowing through HTTP and gRPC servers to hot and warm storage tiers"
                style={{width: '100%', height: 'auto', borderRadius: '12px'}}
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function SDKSection() {
  return (
    <section className={styles.sdkSection}>
      <div className="container">
        <div className="text--center margin-bottom--lg">
          <Heading as="h2">Client SDKs</Heading>
          <p>Native clients for your favorite languages</p>
        </div>
        <div className="row">
          <div className="col col--4">
            <CodeBlock language="go" title="Go">
{`client, _ := feather.NewClient("localhost:8080")

features, _ := client.GetFeatures(ctx,
  "user:123",
  []string{"clicks", "purchases"},
)

fmt.Println(features["clicks"].Value)`}
            </CodeBlock>
          </div>
          <div className="col col--4">
            <CodeBlock language="python" title="Python">
{`from feather import FeatherClient

client = FeatherClient("localhost:8080")

features = client.get_features(
    "user:123",
    ["clicks", "purchases"]
)

print(features["clicks"])`}
            </CodeBlock>
          </div>
          <div className="col col--4">
            <CodeBlock language="bash" title="REST API">
{`curl -X GET \\
  "http://localhost:8080/v1/features\\
?entity=user:123\\
&feature=clicks\\
&feature=purchases"

# Returns JSON with feature values`}
            </CodeBlock>
          </div>
        </div>
      </div>
    </section>
  );
}

type UsedByItem = {
  icon: string;
  category: string;
  description: string;
};

const usedByItems: UsedByItem[] = [
  { icon: '🛒', category: 'E-Commerce', description: 'Personalization & Recommendations' },
  { icon: '🏦', category: 'FinTech', description: 'Real-time Risk Scoring' },
  { icon: '🎮', category: 'Gaming', description: 'Player Behavior Features' },
  { icon: '🚗', category: 'Mobility', description: 'Dynamic Pricing & ETAs' },
];

function UsedBySection() {
  return (
    <section className={styles.usedBySection}>
      <div className="container text--center">
        <Heading as="h2">Built for Production ML</Heading>
        <p className={styles.usedBySubtitle}>
          Feather powers feature serving across industries
        </p>
        <div className={styles.usedByLogos}>
          {usedByItems.map((item, idx) => (
            <div key={idx} className={styles.usedByCard}>
              <div className={styles.usedByIcon}>{item.icon}</div>
              <div className={styles.usedByCategory}>{item.category}</div>
              <div className={styles.usedByDesc}>{item.description}</div>
            </div>
          ))}
        </div>
        <div className={styles.usedByCTA}>
          <p className={styles.usedByNote}>
            Using Feather in production?{' '}
            <Link to="https://github.com/feather-store/feather/discussions">
              Share your story
            </Link>{' '}
            and get featured.
          </p>
        </div>
      </div>
    </section>
  );
}

function CTASection() {
  return (
    <section className={styles.ctaSection}>
      <div className="container text--center">
        <Heading as="h2">Ready to Get Started?</Heading>
        <p>
          Join ML teams serving millions of features with sub-millisecond latency.
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--primary button--lg"
            to="/docs/getting-started">
            Read the Docs
          </Link>
          <Link
            className="button button--outline button--lg"
            to="https://github.com/feather-store/feather"
            style={{marginLeft: '1rem'}}>
            Star on GitHub
          </Link>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="High-Performance Feature Store for ML"
      description="Feather is a high-performance, real-time feature store for machine learning. Sub-millisecond latency, single binary deployment, no external dependencies.">
      <HomepageHeader />
      <main>
        <BuiltForSection />
        <InstallSection />
        <FeaturesSection />
        <PerformanceSection />
        <ArchitectureSection />
        <SDKSection />
        <UsedBySection />
        <CTASection />
      </main>
    </Layout>
  );
}

import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './404.module.css';

export default function NotFound(): ReactNode {
  return (
    <Layout title="Page Not Found">
      <main className={styles.container}>
        <div className={styles.content}>
          <div className={styles.errorCode}>404</div>
          <Heading as="h1" className={styles.title}>
            Page Not Found
          </Heading>
          <p className={styles.description}>
            The page you're looking for doesn't exist or has been moved.
          </p>
          <div className={styles.links}>
            <Link className="button button--primary button--lg" to="/">
              Back to Home
            </Link>
            <Link
              className="button button--outline button--lg"
              to="/docs/getting-started">
              Getting Started
            </Link>
          </div>
          <div className={styles.helpSection}>
            <p className={styles.helpText}>Looking for something specific?</p>
            <ul className={styles.helpLinks}>
              <li>
                <Link to="/docs/api-reference">API Reference</Link>
              </li>
              <li>
                <Link to="/docs/concepts/architecture">Architecture</Link>
              </li>
              <li>
                <Link to="/docs/guides/deployment">Deployment Guide</Link>
              </li>
              <li>
                <Link to="https://github.com/feather-store/feather/issues">
                  Report an Issue
                </Link>
              </li>
            </ul>
          </div>
        </div>
      </main>
    </Layout>
  );
}

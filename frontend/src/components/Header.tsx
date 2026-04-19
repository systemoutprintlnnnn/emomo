import { motion } from 'framer-motion';
import styles from './Header.module.css';

/**
 * Props for the Header component.
 */
interface HeaderProps {
  /**
   * Callback function triggered when the logo is clicked.
   * Typically used to reset the view or navigate home.
   */
  onLogoClick?: () => void;
}

/**
 * The application header component.
 * Displays the logo, statistics, and external links (e.g., GitHub).
 *
 * @param props - The component props.
 * @param props.onLogoClick - Handler for logo click events.
 * @returns The rendered Header component.
 */
export default function Header({ onLogoClick }: HeaderProps) {
  return (
    <motion.header
      className={styles.header}
      initial={{ opacity: 0, y: -20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5 }}
    >
      <div className={styles.container}>
        {/* Logo */}
        <motion.button
          className={styles.logo}
          onClick={onLogoClick}
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
        >
          <span className={styles.logoIcon}>
            <motion.span
              animate={{ rotate: [0, 5, -5, 0] }}
              transition={{ duration: 4, repeat: Infinity }}
            >
              😸
            </motion.span>
          </span>
          <span className={styles.logoText}>Emomo</span>
          <span className={styles.logoBeta}>Beta</span>
        </motion.button>

        {/* Right section */}
        <div className={styles.right}>
          {/* Stats */}
          <div className={styles.stats}>
            <span className={styles.statNumber}>5,791</span>
            <span className={styles.statLabel}>表情包</span>
          </div>

          {/* GitHub link */}
          <motion.a
            href="https://github.com/systemoutprintlnnnn/emomo"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.githubBtn}
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            title="GitHub"
          >
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
          </motion.a>
        </div>
      </div>
    </motion.header>
  );
}

import { motion, AnimatePresence } from 'framer-motion';
import type { Meme } from '../types';
import MemeCard from './MemeCard';
import styles from './MemeGrid.module.css';

/**
 * Props for the MemeGrid component.
 */
interface MemeGridProps {
  /** The list of memes to display in the grid. */
  memes: Meme[];
  /**
   * Indicates whether the grid is in a loading state.
   * If true, displays loading skeletons instead of memes.
   * @default false
   */
  isLoading?: boolean;
  /**
   * Callback function triggered when a meme card is clicked.
   * @param meme - The meme data associated with the clicked card.
   */
  onMemeClick?: (meme: Meme) => void;
  /**
   * Message to display when the meme list is empty.
   * @default '暂无表情包'
   */
  emptyMessage?: string;
  /** The search query string, used to display results information. */
  searchQuery?: string;
  /** An optional title for the grid section (e.g., "Recommended"). */
  title?: string;
}

/**
 * A loading skeleton component for a meme card.
 *
 * @param props - The component props.
 * @param props.index - The index for animation delay.
 * @returns The rendered SkeletonCard component.
 */
function SkeletonCard({ index }: { index: number }) {
  return (
    <motion.div
      className={styles.skeletonCard}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ delay: index * 0.05 }}
    >
      <div className={`${styles.skeletonImage} skeleton`} />
    </motion.div>
  );
}

/**
 * A component that displays a responsive grid of meme cards.
 * Handles loading states, empty states, and section titles.
 *
 * @param props - The component props.
 * @param props.memes - The list of memes to display.
 * @param props.isLoading - Whether the data is loading.
 * @param props.onMemeClick - Handler for meme click events.
 * @param props.emptyMessage - Custom empty state message.
 * @param props.searchQuery - The current search query.
 * @param props.title - Optional section title.
 * @returns The rendered MemeGrid component.
 */
export default function MemeGrid({
  memes,
  isLoading = false,
  onMemeClick,
  emptyMessage = '暂无表情包',
  searchQuery,
  title,
}: MemeGridProps) {
  const scoredMemes = memes.filter((meme) => typeof meme.score === 'number');
  const topScore = scoredMemes.length > 0
    ? Math.max(...scoredMemes.map((meme) => meme.score ?? 0))
    : null;
  const hasLowConfidence = !!searchQuery && topScore !== null && topScore < 0.15;

  // Show loading skeletons
  if (isLoading) {
    return (
      <section className={styles.container}>
        {title && (
          <motion.h2
            className={styles.sectionTitle}
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
          >
            {title}
          </motion.h2>
        )}
        <div className={styles.grid}>
          {Array.from({ length: 12 }).map((_, i) => (
            <SkeletonCard key={i} index={i} />
          ))}
        </div>
      </section>
    );
  }

  // Empty state
  if (memes.length === 0) {
    return (
      <section className={styles.container}>
        <motion.div
          className={styles.empty}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
        >
          <div className={styles.emptyIcon}>
            <motion.div
              animate={{ rotate: [0, 10, -10, 0] }}
              transition={{ duration: 2, repeat: Infinity, repeatDelay: 1 }}
            >
              🤔
            </motion.div>
          </div>
          <h3 className={styles.emptyTitle}>{emptyMessage}</h3>
          {searchQuery && (
            <p className={styles.emptyText}>
              找不到与「{searchQuery}」相关的表情包，试试其他关键词？
            </p>
          )}
        </motion.div>
      </section>
    );
  }

  return (
    <section className={styles.container}>
      <motion.header
        className={styles.resultsHeader}
        initial={{ opacity: 0, y: -10 }}
        animate={{ opacity: 1, y: 0 }}
      >
        {title && (
          <h2 className={styles.sectionTitle}>{title}</h2>
        )}

        {searchQuery && (
          <div className={styles.resultsInfo}>
            <span className={styles.resultsQuery}>「{searchQuery}」</span>
            <span className={styles.resultsCount}>找到 {memes.length} 个表情包</span>
          </div>
        )}

        {hasLowConfidence && (
          <p className={styles.qualityNotice}>
            匹配度偏低，当前结果更像相近情绪或相近语境。
          </p>
        )}
      </motion.header>

      {/* Grid */}
      <motion.div
        className={styles.grid}
        layout
      >
        <AnimatePresence mode="popLayout">
          {memes.map((meme, index) => (
            <MemeCard
              key={meme.id}
              meme={meme}
              index={index}
              onClick={onMemeClick}
            />
          ))}
        </AnimatePresence>
      </motion.div>

      {/* Load more indicator */}
      {memes.length > 0 && (
        <motion.div
          className={styles.endIndicator}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.5 }}
        >
          <span className={styles.endLine} />
          <span className={styles.endText}>已展示全部结果</span>
          <span className={styles.endLine} />
        </motion.div>
      )}
    </section>
  );
}

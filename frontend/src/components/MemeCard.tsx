import { useState } from 'react';
import { motion } from 'framer-motion';
import type { Meme } from '../types';
import styles from './MemeCard.module.css';

/**
 * Props for the MemeCard component.
 */
interface MemeCardProps {
  /** The meme data to display. */
  meme: Meme;
  /**
   * The index of the card in the list, used for staggered animation delays.
   * @default 0
   */
  index?: number;
  /**
   * Callback function triggered when the card is clicked.
   * @param meme - The meme data associated with the card.
   */
  onClick?: (meme: Meme) => void;
}

/**
 * A component that displays a single meme card with an image, hover effects, and quick actions.
 *
 * @param props - The component props.
 * @param props.meme - The meme object containing details like URL, description, etc.
 * @param props.index - The index for animation timing.
 * @param props.onClick - The click handler for the card.
 * @returns The rendered MemeCard component.
 */
export default function MemeCard({ meme, index = 0, onClick }: MemeCardProps) {
  const [isLoaded, setIsLoaded] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [imageError, setImageError] = useState(false);

  const handleClick = () => {
    onClick?.(meme);
  };

  const handleImageError = () => {
    setImageError(true);
    setIsLoaded(true); // Stop showing skeleton
  };

  // Format score as percentage
  const scorePercent = meme.score ? Math.round(meme.score * 100) : null;

  return (
    <motion.div
      className={styles.card}
      initial={{ opacity: 0, y: 20, scale: 0.95 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{
        duration: 0.4,
        delay: index * 0.05,
        ease: [0.16, 1, 0.3, 1],
      }}
      whileHover={{ y: -4 }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onClick={handleClick}
      layout
    >
      {/* Image container */}
      <div className={styles.imageWrapper}>
        {/* Loading skeleton */}
        {!isLoaded && !imageError && <div className={`${styles.skeleton} skeleton`} />}

        {/* Error placeholder */}
        {imageError && (
          <div className={styles.imageError}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
          </div>
        )}

        {/* Meme image */}
        {!imageError && (
          <motion.img
            src={meme.url || meme.original_url}
            alt={meme.vlm_description || 'Meme'}
            className={styles.image}
            loading="lazy"
            onLoad={() => setIsLoaded(true)}
            onError={handleImageError}
            animate={{
              scale: isHovered ? 1.05 : 1,
              opacity: isLoaded ? 1 : 0,
            }}
            transition={{ duration: 0.3 }}
          />
        )}

        {/* Hover overlay */}
        <motion.div
          className={styles.overlay}
          initial={{ opacity: 0 }}
          animate={{ opacity: isHovered ? 1 : 0 }}
          transition={{ duration: 0.2 }}
        >
          <div className={styles.overlayContent}>
            {/* Quick actions */}
            <div className={styles.actions}>
              <motion.button
                className={styles.actionBtn}
                whileHover={{ scale: 1.1 }}
                whileTap={{ scale: 0.9 }}
                onClick={(e) => {
                  e.stopPropagation();
                  // Copy image URL
                  if (meme.url) {
                    navigator.clipboard.writeText(meme.url);
                  }
                }}
                title="复制链接"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="9" y="9" width="13" height="13" rx="2" />
                  <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
                </svg>
              </motion.button>
              <motion.button
                className={styles.actionBtn}
                whileHover={{ scale: 1.1 }}
                whileTap={{ scale: 0.9 }}
                onClick={(e) => {
                  e.stopPropagation();
                  // Download image
                  if (meme.url) {
                    const a = document.createElement('a');
                    a.href = meme.url;
                    a.download = `meme-${meme.id}.${meme.format || 'jpg'}`;
                    a.click();
                  }
                }}
                title="下载"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
                  <polyline points="7,10 12,15 17,10" />
                  <line x1="12" y1="15" x2="12" y2="3" />
                </svg>
              </motion.button>
            </div>

            {/* Description preview */}
            {meme.vlm_description && (
              <p className={styles.description}>
                {meme.vlm_description.slice(0, 60)}
                {meme.vlm_description.length > 60 ? '...' : ''}
              </p>
            )}
          </div>
        </motion.div>

        {/* Animated badge for GIFs */}
        {meme.is_animated && (
          <span className={styles.gifBadge}>GIF</span>
        )}

        {/* Score badge */}
        {scorePercent !== null && (
          <motion.div
            className={styles.scoreBadge}
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ delay: index * 0.05 + 0.2, type: 'spring' }}
          >
            {scorePercent}%
          </motion.div>
        )}
      </div>
    </motion.div>
  );
}

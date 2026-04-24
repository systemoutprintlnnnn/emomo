import { useState, useCallback, useEffect, useRef } from 'react';
import { AnimatePresence } from 'framer-motion';
import { Header, SearchHero, MemeGrid, MemeModal } from './components';
import SearchProgress from './components/SearchProgress';
import {
  searchMemesStream,
  getMemes,
  getStats,
  type SearchStage,
  type SearchProgressEvent,
} from './api';
import { curatedMemes } from './data/curatedMemes';
import type { Meme } from './types';
import './App.css';

// Search state for streaming progress
interface SearchState {
  isStreaming: boolean;
  stage: SearchStage;
  message: string;
  thinkingText: string;
  expandedQuery?: string;
}

function App() {
  const [memes, setMemes] = useState<Meme[]>([]);
  const [recommendedMemes, setRecommendedMemes] = useState<Meme[]>(curatedMemes);
  const [isLoading, setIsLoading] = useState(false);
  const [inputQuery, setInputQuery] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [memeCount, setMemeCount] = useState(5791);
  const [selectedMeme, setSelectedMeme] = useState<Meme | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [searchState, setSearchState] = useState<SearchState | null>(null);
  const hasFetchedRef = useRef(false);
  const abortControllerRef = useRef<AbortController | null>(null);

  // 加载推荐表情（首屏）
  useEffect(() => {
    if (hasFetchedRef.current) return;
    hasFetchedRef.current = true;

    const loadRecommendedMemes = async () => {
      try {
        const response = await getMemes(12, 0);
        if (response.results.length > 0) {
          setRecommendedMemes(response.results);
        }
      } catch (error) {
        console.error('Failed to load recommended memes:', error);
      }
    };

    const loadStats = async () => {
      try {
        const stats = await getStats();
        if (stats.total_active > 0) {
          setMemeCount(stats.total_active);
        }
      } catch (error) {
        console.error('Failed to load stats:', error);
      }
    };

    loadRecommendedMemes();
    loadStats();
  }, []);

  // Handle cancel search
  const handleCancelSearch = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    setSearchState(null);
    setIsLoading(false);
  }, []);

  // Handle search with streaming progress
  const handleSearch = useCallback(async (query: string) => {
    // Cancel any existing search
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    setInputQuery(query);
    setSearchQuery(query);
    setIsLoading(true);
    setHasSearched(true);
    setMemes([]); // Clear previous results

    // Initialize search state
    setSearchState({
      isStreaming: true,
      stage: 'query_expansion_start',
      message: 'AI 正在理解搜索意图...',
      thinkingText: '',
    });

    // Accumulate thinking text
    let accumulatedThinking = '';

    try {
      await searchMemesStream(
        query,
        20,
        (event: SearchProgressEvent) => {
          if (abortController.signal.aborted) {
            return;
          }

          // Update search state based on event
          if (event.stage === 'thinking') {
            // Accumulate thinking text for typewriter effect
            if (event.is_delta && event.thinking_text) {
              accumulatedThinking += event.thinking_text;
              setSearchState((prev) =>
                prev
                  ? {
                      ...prev,
                      stage: 'thinking',
                      thinkingText: accumulatedThinking,
                    }
                  : null
              );
            }
          } else if (event.stage === 'complete') {
            // Search complete - update results
            if (event.results) {
              setMemes(event.results);
            }
            setSearchState(null);
            setIsLoading(false);
          } else if (event.stage === 'error') {
            console.error('Search error:', event.error);
            setSearchState(null);
            setIsLoading(false);
            // Use curated data as fallback
            const filtered = curatedMemes.filter(
              (m) =>
                m.description?.toLowerCase().includes(query.toLowerCase()) ||
                m.vlm_description?.toLowerCase().includes(query.toLowerCase()) ||
                m.tags?.some((t) => t.toLowerCase().includes(query.toLowerCase())) ||
                m.category?.toLowerCase().includes(query.toLowerCase())
            );
            setMemes(filtered.length > 0 ? filtered : curatedMemes);
          } else {
            // Progress update
            setSearchState((prev) =>
              prev
                ? {
                    ...prev,
                    stage: event.stage,
                    message: event.message || prev.message,
                    expandedQuery: event.expanded_query || prev.expandedQuery,
                  }
                : null
            );
          }
        },
        abortController.signal
      );
    } catch (error) {
      if ((error as Error).name === 'AbortError') {
        // Search was cancelled - do nothing
        return;
      }
      console.error('Search failed:', error);
      setSearchState(null);
      // Use curated data as fallback
      const filtered = curatedMemes.filter(
        (m) =>
          m.description?.toLowerCase().includes(query.toLowerCase()) ||
          m.vlm_description?.toLowerCase().includes(query.toLowerCase()) ||
          m.tags?.some((t) => t.toLowerCase().includes(query.toLowerCase())) ||
          m.category?.toLowerCase().includes(query.toLowerCase())
      );
      setMemes(filtered.length > 0 ? filtered : curatedMemes);
    } finally {
      if (abortControllerRef.current === abortController) {
        abortControllerRef.current = null;
        setIsLoading(false);
      }
    }
  }, []);

  // Handle logo click - reset to home
  const handleLogoClick = useCallback(() => {
    // Cancel any ongoing search
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    setMemes([]);
    setInputQuery('');
    setSearchQuery('');
    setHasSearched(false);
    setSelectedMeme(null);
    setSearchState(null);
    setIsLoading(false);
  }, []);

  // Handle meme click
  const handleMemeClick = useCallback((meme: Meme) => {
    setSelectedMeme(meme);
  }, []);

  // Handle modal close
  const handleModalClose = useCallback(() => {
    setSelectedMeme(null);
  }, []);

  return (
    <div className="app">
      <Header memeCount={memeCount} onLogoClick={handleLogoClick} />

      <main className="main">
        <SearchHero
          value={inputQuery}
          onValueChange={setInputQuery}
          onSearch={handleSearch}
          isLoading={isLoading}
          compact={hasSearched}
        />

        {/* Search Progress */}
        <AnimatePresence>
          {searchState?.isStreaming && (
            <SearchProgress
              stage={searchState.stage}
              message={searchState.message}
              thinkingText={searchState.thinkingText}
              expandedQuery={searchState.expandedQuery}
              onCancel={handleCancelSearch}
            />
          )}
        </AnimatePresence>

        {hasSearched ? (
          <MemeGrid
            memes={memes}
            isLoading={isLoading || !!searchState?.isStreaming}
            onMemeClick={handleMemeClick}
            searchQuery={searchQuery}
            emptyMessage="没有找到相关表情包"
          />
        ) : (
          <MemeGrid
            memes={recommendedMemes}
            isLoading={false}
            onMemeClick={handleMemeClick}
            searchQuery=""
            emptyMessage=""
            title="推荐表情"
          />
        )}
      </main>

      <MemeModal
        meme={selectedMeme}
        isOpen={!!selectedMeme}
        onClose={handleModalClose}
      />
    </div>
  );
}

export default App;

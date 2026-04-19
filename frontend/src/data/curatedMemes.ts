import type { Meme } from '../types';
import { curatedMemesData } from './curatedMemesData';

export const curatedMemes: Meme[] = curatedMemesData.map((item) => ({
  id: item.id,
  url: item.url,
  original_url: item.url,
  width: item.width,
  height: item.height,
  format: item.format,
  is_animated: item.is_animated ?? false,
  vlm_description: item.description,
  description: item.description,
  tags: item.tags,
  category: item.category,
}));

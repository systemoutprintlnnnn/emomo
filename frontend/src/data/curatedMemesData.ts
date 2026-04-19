export interface CuratedMemeDisplayData {
  id: string;
  url: string;
  width?: number;
  height?: number;
  format?: string;
  is_animated?: boolean;
  description?: string;
  tags?: string[];
  category?: string;
}

export const curatedMemesData: CuratedMemeDisplayData[] = [
  {
    id: 'curated-1',
    url: 'https://images.unsplash.com/photo-1514888286974-6c03e2ca1dba?w=300&h=300&fit=crop',
    width: 300,
    height: 300,
    format: 'jpg',
    is_animated: false,
    description: '一只橘色的猫咪，眼神慵懒地看着镜头，非常可爱',
    tags: ['猫', '可爱', '橘猫'],
    category: '猫咪',
  },
  {
    id: 'curated-2',
    url: 'https://images.unsplash.com/photo-1543466835-00a7907e9de1?w=300&h=400&fit=crop',
    width: 300,
    height: 400,
    format: 'jpg',
    is_animated: false,
    description: '一只金毛犬，张大嘴巴像在笑，非常开心的样子',
    tags: ['狗', '开心', '金毛'],
    category: '狗狗',
  },
  {
    id: 'curated-3',
    url: 'https://images.unsplash.com/photo-1518791841217-8f162f1e1131?w=350&h=250&fit=crop',
    width: 350,
    height: 250,
    format: 'jpg',
    is_animated: false,
    description: '一只蓝色眼睛的白猫，优雅地坐着',
    tags: ['猫', '白猫', '优雅'],
    category: '猫咪',
  },
  {
    id: 'curated-4',
    url: 'https://images.unsplash.com/photo-1587300003388-59208cc962cb?w=280&h=280&fit=crop',
    width: 280,
    height: 280,
    format: 'gif',
    is_animated: true,
    description: '一只哈士奇，歪着头看镜头，表情很滑稽',
    tags: ['狗', '哈士奇', '滑稽'],
    category: '狗狗',
  },
  {
    id: 'curated-5',
    url: 'https://images.unsplash.com/photo-1574158622682-e40e69881006?w=320&h=380&fit=crop',
    width: 320,
    height: 380,
    format: 'jpg',
    is_animated: false,
    description: '一只黑白相间的猫，睁大眼睛，表情很惊讶',
    tags: ['猫', '惊讶', '黑白猫'],
    category: '猫咪',
  },
  {
    id: 'curated-6',
    url: 'https://images.unsplash.com/photo-1552053831-71594a27632d?w=300&h=350&fit=crop',
    width: 300,
    height: 350,
    format: 'jpg',
    is_animated: false,
    description: '一只金毛幼犬，趴在地上，眼神无辜',
    tags: ['狗', '幼犬', '无辜'],
    category: '狗狗',
  },
  {
    id: 'curated-7',
    url: 'https://images.unsplash.com/photo-1573865526739-10659fec78a5?w=280&h=320&fit=crop',
    width: 280,
    height: 320,
    format: 'jpg',
    is_animated: false,
    description: '一只橘猫，闭着眼睛，看起来很满足',
    tags: ['猫', '满足', '睡觉'],
    category: '猫咪',
  },
  {
    id: 'curated-8',
    url: 'https://images.unsplash.com/photo-1583511655857-d19b40a7a54e?w=340&h=280&fit=crop',
    width: 340,
    height: 280,
    format: 'jpg',
    is_animated: false,
    description: '一只柴犬，咧嘴笑，表情很开心',
    tags: ['狗', '柴犬', '笑'],
    category: '狗狗',
  },
];

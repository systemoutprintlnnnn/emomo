import os
import re
import time
import random
import hashlib
import requests
import ssl
from urllib.parse import urljoin, urlparse
from bs4 import BeautifulSoup
from concurrent.futures import ThreadPoolExecutor, as_completed
from requests.adapters import HTTPAdapter
from urllib3.util.ssl_ import create_urllib3_context

# ============ 配置区域 ============
BASE_URL = "https://fabiaoqing.com"
SAVE_DIR = "./emoji_images"  # 修改此路径来更改保存位置
MAX_PAGES = 5  # 每个分类爬取的页数
MAX_IMAGES = 100  # 最大下载图片数
DELAY_RANGE = (0.5, 1.5)  # 请求延迟范围(秒)
THREADS = 5  # 并发下载线程数
# ==================================

# 请求头，模拟浏览器
HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'Accept-Encoding': 'gzip, deflate, br',
    'Connection': 'keep-alive',
    'Referer': BASE_URL,
}

class TLSAdapter(HTTPAdapter):
    """自定义适配器，强制使用特定的 TLS 版本和密码学套件"""
    def init_poolmanager(self, *args, **kwargs):
        # 使用 ssl.create_default_context() 匹配 test_urllib3.py 的成功配置
        context = ssl.create_default_context()
        context.set_ciphers('DEFAULT@SECLEVEL=1')
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.maximum_version = ssl.TLSVersion.TLSv1_2
        # 跳过证书验证（如 verify=False 时所需）
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        
        kwargs['ssl_context'] = context
        return super(TLSAdapter, self).init_poolmanager(*args, **kwargs)

    def proxy_manager_for(self, *args, **kwargs):
        context = ssl.create_default_context()
        context.set_ciphers('DEFAULT@SECLEVEL=1')
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.maximum_version = ssl.TLSVersion.TLSv1_2
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        
        kwargs['ssl_context'] = context
        return super(TLSAdapter, self).proxy_manager_for(*args, **kwargs)

def create_session():
    """创建带有 SSL 修复和重试机制的会话"""
    session = requests.Session()
    session.headers.update(HEADERS)
    session.mount("https://", TLSAdapter())
    session.verify = False  # 跳过SSL验证
    return session

def get_page_content(session, url):
    """获取页面内容"""
    try:
        time.sleep(random.uniform(*DELAY_RANGE))
        response = session.get(url, timeout=15)
        response.raise_for_status()
        response.encoding = response.apparent_encoding or 'utf-8'
        return response.text
    except requests.RequestException as e:
        print(f"获取页面失败 {url}: {e}")
        return None

def parse_emoji_list(html, base_url):
    """解析表情包列表页面，提取图片URL"""
    soup = BeautifulSoup(html, 'html.parser')
    images = []
    
    # 尝试多种选择器来匹配图片
    selectors = [
        'img.ui.image.lazy',  # 懒加载图片
        'img[data-original]',  # data-original属性
        '.bqppdiv img',  # 表情包容器
        '.tagbqppdiv img',  # 标签表情包
        '.bqba img',  # 另一种容器
        'img.lazy',  # 懒加载类
        '.image img',  # image类容器
        'a.image img',  # 链接中的图片
    ]
    
    found_imgs = set()
    for selector in selectors:
        for img in soup.select(selector):
            # 优先获取data-original（懒加载真实地址）
            src = img.get('data-original') or img.get('src') or img.get('data-src')
            if src:
                # 过滤掉占位图和图标
                if 'placeholder' in src.lower() or 'icon' in src.lower():
                    continue
                if src.startswith('//'):
                    src = 'https:' + src
                elif src.startswith('/'):
                    src = urljoin(base_url, src)
                # 只保留图片文件
                if any(ext in src.lower() for ext in ['.gif', '.jpg', '.jpeg', '.png', '.webp']):
                    found_imgs.add(src)
    
    images = list(found_imgs)
    print(f"在页面中找到 {len(images)} 张图片")
    return images

def parse_detail_page(html, base_url):
    """解析详情页面获取高清图片"""
    soup = BeautifulSoup(html, 'html.parser')
    images = []
    
    # 详情页可能有更大的图片
    for img in soup.select('img'):
        src = img.get('data-original') or img.get('src') or img.get('data-src')
        if src:
            if src.startswith('//'):
                src = 'https:' + src
            elif src.startswith('/'):
                src = urljoin(base_url, src)
            if any(ext in src.lower() for ext in ['.gif', '.jpg', '.jpeg', '.png', '.webp']):
                if 'placeholder' not in src.lower():
                    images.append(src)
    
    return images

def get_category_urls(session):
    """获取分类页面URL列表"""
    urls = [
        f"{BASE_URL}/biaoqing",  # 表情包列表
        f"{BASE_URL}/zui/hot",   # 热门
        f"{BASE_URL}/zui/new",   # 最新
    ]
    
    # 尝试获取更多分类
    html = get_page_content(session, BASE_URL)
    if html:
        soup = BeautifulSoup(html, 'html.parser')
        for a in soup.select('a[href*="/search/bqb/"]'):
            href = a.get('href')
            if href:
                if href.startswith('/'):
                    href = urljoin(BASE_URL, href)
                urls.append(href)
    
    return list(set(urls))[:5]  # 限制分类数量

def download_image(session, url, save_dir, index):
    """下载单张图片"""
    try:
        # 从URL提取文件名
        parsed = urlparse(url)
        filename = os.path.basename(parsed.path)
        
        # 确保有正确的扩展名
        if not filename or '.' not in filename:
            ext = '.gif' if '.gif' in url.lower() else '.jpg'
            filename = f"emoji_{index:04d}{ext}"
        else:
            # 清理文件名中的特殊字符
            filename = re.sub(r'[^\w\-.]', '_', filename)
        
        filepath = os.path.join(save_dir, filename)
        
        # 下载图片
        response = session.get(url, timeout=15)
        response.raise_for_status()
        
        # 使用MD5检查是否已存在相同内容
        content_hash = hashlib.md5(response.content).hexdigest()[:8]
        
        # 如果文件已存在，添加hash后缀
        if os.path.exists(filepath):
            name, ext = os.path.splitext(filename)
            filepath = os.path.join(save_dir, f"{name}_{content_hash}{ext}")
        
        with open(filepath, 'wb') as f:
            f.write(response.content)
        
        size_kb = len(response.content) / 1024
        print(f"✓ 下载成功: {os.path.basename(filepath)} ({size_kb:.1f}KB)")
        return filepath
    except Exception as e:
        print(f"✗ 下载失败 {url}: {e}")
        return None

def main():
    """主函数"""
    # 禁用SSL警告
    import urllib3
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
    
    print("=" * 50)
    print("  发表情 (fabiaoqing.com) 表情包爬虫")
    print("=" * 50)
    
    # 创建保存目录
    os.makedirs(SAVE_DIR, exist_ok=True)
    print(f"\n📁 图片将保存到: {os.path.abspath(SAVE_DIR)}")
    
    session = create_session()
    all_image_urls = set()
    
    # 1. 获取分类URL
    print("\n🔍 获取分类页面...")
    category_urls = get_category_urls(session)
    print(f"找到 {len(category_urls)} 个分类页面")
    
    # 2. 遍历分类页面收集图片URL
    for cat_url in category_urls:
        print(f"\n📂 处理分类: {cat_url}")
        
        for page in range(1, MAX_PAGES + 1):
            if len(all_image_urls) >= MAX_IMAGES:
                break
            
            page_url = f"{cat_url}?page={page}" if page > 1 else cat_url
            print(f"  第 {page} 页...", end=" ")
            
            html = get_page_content(session, page_url)
            if not html:
                print("获取失败")
                break
            
            images = parse_emoji_list(html, BASE_URL)
            if not images:
                print("无更多图片")
                break
            
            all_image_urls.update(images)
            print(f"找到 {len(images)} 张, 累计 {len(all_image_urls)} 张")
        
        if len(all_image_urls) >= MAX_IMAGES:
            print(f"\n已达到最大数量限制 ({MAX_IMAGES})")
            break
    
    # 3. 并发下载图片
    image_urls = list(all_image_urls)[:MAX_IMAGES]
    print(f"\n⬇️  开始下载 {len(image_urls)} 张图片 (并发数: {THREADS})...")
    print("-" * 50)
    
    downloaded = []
    with ThreadPoolExecutor(max_workers=THREADS) as executor:
        futures = {
            executor.submit(download_image, session, url, SAVE_DIR, i): url
            for i, url in enumerate(image_urls, 1)
        }
        for future in as_completed(futures):
            result = future.result()
            if result:
                downloaded.append(result)
    
    # 4. 统计结果
    print("\n" + "=" * 50)
    print(f"✅ 下载完成!")
    print(f"   成功: {len(downloaded)} 张")
    print(f"   失败: {len(image_urls) - len(downloaded)} 张")
    print(f"   保存位置: {os.path.abspath(SAVE_DIR)}")
    print("=" * 50)
    
    # 列出文件类型统计
    if downloaded:
        exts = {}
        for f in downloaded:
            ext = os.path.splitext(f)[1].lower()
            exts[ext] = exts.get(ext, 0) + 1
        print("\n📊 文件类型统计:")
        for ext, count in sorted(exts.items(), key=lambda x: -x[1]):
            print(f"   {ext}: {count} 张")
    
    return downloaded

if __name__ == "__main__":
    main()

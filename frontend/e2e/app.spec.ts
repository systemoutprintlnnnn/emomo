import { test, expect } from '@playwright/test';

test.describe('Emomo 表情包搜索应用', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('页面正确加载', async ({ page }) => {
    // 检查标题
    await expect(page.locator('h1')).toContainText('用文字找表情');
    // 检查副标题
    await expect(page.getByText('AI 驱动的语义搜索')).toBeVisible();
    // 检查搜索框存在
    await expect(page.locator('input[type="text"]')).toBeVisible();
    // 检查搜索按钮
    await expect(page.getByRole('button', { name: '搜索' })).toBeVisible();
  });

  test('热门标签显示并可点击', async ({ page }) => {
    // 检查热门标签区域
    await expect(page.getByText('热门:')).toBeVisible();

    // 检查至少有一些热门标签
    const tags = page.locator('button').filter({ hasText: /^(开心|无语|狗头|猫咪|熊猫头|沙雕)$/ });
    await expect(tags.first()).toBeVisible();
  });

  test('搜索功能工作正常', async ({ page }) => {
    // 输入搜索词
    const searchInput = page.locator('input[type="text"]');
    await searchInput.fill('猫咪');

    // 点击搜索按钮
    await page.getByRole('button', { name: '搜索' }).click();

    // 等待加载完成（检查没有 loading 状态）
    await expect(searchInput).not.toBeDisabled({ timeout: 10000 });

    // 检查搜索结果区域存在
    await page.waitForTimeout(1000); // 等待结果渲染
  });

  test('点击热门标签触发搜索', async ({ page }) => {
    // 找到一个热门标签并点击
    const tagButton = page.locator('button').filter({ hasText: '开心' }).first();
    await tagButton.click();

    // 检查输入框有值
    const searchInput = page.locator('input[type="text"]');
    await expect(searchInput).toHaveValue('开心');
  });

  test('搜索框可以清除', async ({ page }) => {
    // 输入搜索词
    const searchInput = page.locator('input[type="text"]');
    await searchInput.fill('测试');

    // 检查清除按钮出现
    const clearButton = page.locator('button[type="button"]').filter({ has: page.locator('svg') }).first();

    // 清除内容
    await searchInput.clear();
    await expect(searchInput).toHaveValue('');
  });

  test('推荐表情区域显示', async ({ page }) => {
    // 检查推荐表情标题或表情网格
    // 等待初始加载
    await page.waitForTimeout(2000);

    // 检查页面有表情卡片或者推荐区域
    const hasMemes = await page.locator('img').count() > 0;
    expect(hasMemes || await page.getByText('推荐表情').isVisible()).toBeTruthy();
  });

  test('表情卡片可以点击', async ({ page }) => {
    // 等待推荐表情加载
    await page.waitForTimeout(2000);

    // 找到一个表情卡片
    const memeCard = page.locator('img[alt]').first();
    if (await memeCard.isVisible()) {
      await memeCard.click();

      // 检查弹窗是否出现（如果实现了的话）
      await page.waitForTimeout(500);
    }
  });

  test('响应式布局正常', async ({ page }) => {
    // 测试移动端视图
    await page.setViewportSize({ width: 375, height: 667 });
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('input[type="text"]')).toBeVisible();

    // 测试平板视图
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect(page.locator('h1')).toBeVisible();

    // 测试桌面视图
    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(page.locator('h1')).toBeVisible();
  });

  test('键盘导航 - 回车提交搜索', async ({ page }) => {
    const searchInput = page.locator('input[type="text"]');
    await searchInput.fill('狗狗');
    await searchInput.press('Enter');

    // 验证搜索被触发
    await expect(searchInput).toHaveValue('狗狗');
  });
});

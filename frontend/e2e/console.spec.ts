import { expect, test, type Page } from '@playwright/test';

const ids = {
  platform: 'scope-platform',
  team: 'scope-team',
  project: 'scope-project',
  provider: 'provider-1',
  skill: 'skill-1',
  session: 'diagnosis-session'
};

const user = {
  id: 'user-1',
  username: 'acceptance-admin',
  email: 'admin@example.test',
  display_name: '验收管理员',
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
};

function pageData(page: Page, requireLogin = false, platformAdmin = false) {
  let authenticated = !requireLogin;
  let preferences = {
    theme: 'auto',
    sidebar_mode: 'fixed',
    sidebar_collapsed: false
  };
  return page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const json = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(body)
      });
    if (path.endsWith('/auth/bootstrap')) return json({ required: false });
    if (path.endsWith('/auth/me/context'))
      return json({
        platform_admin: platformAdmin,
        platform_role: platformAdmin
      });
    if (path.endsWith('/auth/me'))
      return authenticated
        ? json(user)
        : json({ code: 'invalid_session' }, 401);
    if (path.endsWith('/auth/login')) {
      authenticated = true;
      return json(user);
    }
    if (path.endsWith('/auth/logout')) return json({}, 204);
    if (path.endsWith('/auth/me/preferences')) {
      if (request.method() === 'PUT') {
        preferences = JSON.parse(request.postData() || '{}');
      }
      return json(preferences);
    }
    if (path.endsWith('/platform'))
      return json({
        id: 'platform-1',
        scope: { id: ids.platform, type: 'platform', status: 'active' },
        name: '验收平台',
        code: 'acceptance',
        status: 'active',
        created_at: user.created_at,
        updated_at: user.updated_at
      });
    if (path.endsWith('/teams/') && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      return json(
        {
          id: 'team-created',
          platform_id: 'platform-1',
          scope: {
            id: 'scope-team-created',
            type: 'team',
            parent_id: ids.platform,
            status: 'active'
          },
          name: body.name,
          description: body.description,
          icon: body.icon,
          labels: body.labels ?? {},
          status: 'active',
          created_at: user.created_at,
          updated_at: user.updated_at
        },
        201
      );
    }
    if (path.endsWith('/teams/'))
      return json({
        items: [
          {
            id: 'team-1',
            platform_id: 'platform-1',
            scope: {
              id: ids.team,
              type: 'team',
              parent_id: ids.platform,
              status: 'active'
            },
            name: '平台工程',
            description: '平台基础设施团队',
            icon: 'lucide:UsersRound',
            labels: {},
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          }
        ],
        page: 1,
        page_size: 100,
        total: 1
      });
    if (path.endsWith('/workspace'))
      return json({
        summary: {
          project_id: 'project-1',
          applications: 0,
          instances: 0,
          resources: 0,
          dependencies: 0,
          alerts: 0
        },
        resources: [],
        alerts: [],
        applications: []
      });
    if (path.includes('/projects'))
      return json({
        items: [
          {
            id: 'project-1',
            platform_id: 'platform-1',
            team_id: 'team-1',
            scope: {
              id: ids.project,
              type: 'project',
              parent_id: ids.team,
              status: 'active'
            },
            name: '支付服务',
            code: 'payments',
            icon: 'lucide:FolderKanban',
            labels: {},
            source: 'manual',
            source_config: {},
            summary: {
              applications: 0,
              applications_healthy: 0,
              instances: 0,
              instances_healthy: 0,
              resources: 0,
              alerts: 0,
              dependencies: 0
            },
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          }
        ],
        page: 1,
        page_size: 100,
        total: 1
      });
    if (path.endsWith('/resources/schemas')) return json([]);
    if (path.includes('/connection-tests/latest'))
      return json({ code: 'not_found' }, 404);
    if (path.endsWith('/resources'))
      return json({ items: [], page: 1, page_size: 100, total: 0 });
    if (path.endsWith('/providers'))
      return json({
        items: [
          {
            id: ids.provider,
            scope_id: ids.platform,
            name: 'Test Provider',
            status: 'active',
            config: {
              provider_type: 'openai_compatible',
              base_url: 'https://llm.test',
              models: [{ name: 'test-model', context_window: 8192 }]
            }
          }
        ]
      });
    if (path.endsWith('/providers/available')) return json([]);
    if (path.endsWith('/skills'))
      return json({
        items: [
          {
            id: ids.skill,
            scope_id: ids.platform,
            name: 'Default Diagnostic',
            identifier: 'default-diagnostic',
            category: 'diagnosis',
            tags: [],
            maintainer: 'OpsSeek',
            status: 'active'
          }
        ]
      });
    if (path.endsWith('/personas'))
      return json({
        items: [
          {
            id: 'persona-1',
            scope_id: ids.platform,
            name: 'Default Diagnostic',
            description: 'test',
            config: {},
            status: 'active'
          }
        ]
      });
    if (path.includes('/personas/') && path.endsWith('/versions'))
      return json([
        {
          id: 'persona-version-1',
          persona_id: 'persona-1',
          version: 1,
          config: {
            version: 1,
            instruction: 'return JSON',
            enabled: true
          },
          status: 'published',
          created_at: user.created_at
        }
      ]);
    if (path.includes('/provider-bindings')) return json([]);
    if (path.includes('/skills/') && path.endsWith('/versions'))
      return json([
        {
          id: 'skill-version-1',
          skill_id: ids.skill,
          version: 1,
          manifest: {
            name: 'Default Diagnostic',
            description: 'test',
            instruction: 'return JSON'
          },
          input_schema: { type: 'object' },
          output_schema: { type: 'object' },
          tools: [],
          risk_level: 'read_only',
          status: 'published',
          created_at: user.created_at
        }
      ]);
    if (path.includes('/diagnosis-sessions/') && path.endsWith('/events'))
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: ''
      });
    if (path.endsWith('/diagnosis-sessions'))
      return json([
        {
          id: ids.session,
          scope_id: ids.project,
          title: '支付服务诊断',
          status: 'succeeded',
          question: '检查健康状态',
          created_at: user.created_at,
          updated_at: user.updated_at
        }
      ]);
    if (path.includes('/diagnosis-sessions/'))
      return json({
        session: {
          id: ids.session,
          scope_id: ids.project,
          title: '支付服务诊断',
          status: 'succeeded',
          question: '检查健康状态',
          created_at: user.created_at,
          updated_at: user.updated_at
        },
        targets: [],
        messages: [],
        plan: [],
        events: [],
        evidence: [],
        hypotheses: [],
        report: null
      });
    if (path.endsWith('/users/') && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      return json(
        {
          user: {
            ...user,
            id: 'user-created',
            username: body.username,
            display_name: body.display_name || body.username,
            email: body.email,
            phone: body.phone,
            can_manage: true
          },
          bindings: [],
          one_time_password: 'GeneratedPassword123!'
        },
        201
      );
    }
    if (path.endsWith('/users/'))
      return json([
        { ...user, can_manage: false },
        {
          ...user,
          id: 'user-2',
          username: 'payment-owner',
          email: 'owner@example.test',
          display_name: '支付负责人',
          can_manage: true
        }
      ]);
    if (path.endsWith('/groups/'))
      return json([
        {
          id: 'group-1',
          scope_id: ids.team,
          name: '平台工程成员',
          description: '平台工程团队',
          status: 'active',
          created_at: user.created_at,
          updated_at: user.updated_at
        }
      ]);
    if (path.endsWith('/groups/group-1/members'))
      return json([
        { group_id: 'group-1', user_id: 'user-2', created_at: user.created_at }
      ]);
    if (path.endsWith('/roles/'))
      return json([
        {
          id: 'role-team-owner',
          name: 'TeamOwner',
          scope_type: 'team',
          builtin: true,
          permissions: ['organization:read', 'member:grant', 'project:manage']
        },
        {
          id: 'role-platform-viewer',
          name: 'PlatformViewer',
          scope_type: 'platform',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        },
        {
          id: 'role-team-viewer',
          name: 'TeamViewer',
          scope_type: 'team',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        },
        {
          id: 'role-project-viewer',
          name: 'ProjectViewer',
          scope_type: 'project',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        }
      ]);
    if (path.endsWith('/role-bindings/'))
      return json([
        {
          id: 'binding-1',
          subject_type: 'user',
          subject_id: 'user-2',
          role_id: 'role-team-owner',
          role_name: 'TeamOwner',
          scope_id: ids.team,
          scope_type: 'team',
          created_at: user.created_at
        }
      ]);
    if (
      path.endsWith('/resource-roles/') ||
      path.endsWith('/resource-role-bindings/')
    )
      return json([]);
    if (path.endsWith('/inspection-policies') && request.method() === 'POST')
      return json(
        {
          id: 'policy-1',
          scope_id: ids.platform,
          name: 'Hourly health',
          cron: '0 * * * *',
          timezone: 'UTC',
          status: 'active',
          target_resource_ids: [],
          target_labels: { env: 'prod' },
          persona_id: '',
          timeout: 120000000000,
          retries: 1,
          max_concurrent: 2,
          max_tool_calls: 12,
          max_tokens: 20000
        },
        201
      );
    if (path.endsWith('/notification-channels') && request.method() === 'POST')
      return json(
        {
          id: 'channel-1',
          scope_id: ids.platform,
          name: 'Incident webhook',
          kind: 'webhook',
          webhook_url: 'https://hooks.example.test/incident',
          status: 'active',
          rate_limit_per_minute: 30
        },
        201
      );
    if (
      path.includes('/inspection-policies') ||
      path.includes('/inspection-runs') ||
      path.includes('/inspection-findings') ||
      path.includes('/notification-channels')
    )
      return json([]);
    return json({}, 200);
  });
}

test.describe('T07 console', () => {
  test('toggles the sidebar with floating boundary chevrons', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('./');

    const sidebar = page.locator('.sidebar');
    const toggle = page.getByRole('button', { name: '折叠导航栏' });
    const username = page.locator('.user-menu-name');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const expanded = await page.evaluate(() => {
      const edge = document
        .querySelector('.sidebar')!
        .getBoundingClientRect().right;
      const control = document
        .querySelector('.sidebar-toggle')!
        .getBoundingClientRect();
      const name = document
        .querySelector('.user-menu-name')!
        .getBoundingClientRect();
      const footer = document
        .querySelector('.sidebar-footer')!
        .getBoundingClientRect();
      const workspace = document
        .querySelector('.workspace-label')!
        .getBoundingClientRect();
      const titlebar = document
        .querySelector('.topbar')!
        .getBoundingClientRect();
      const firstItem = document
        .querySelector('.nav-item')!
        .getBoundingClientRect();
      const firstIcon = document
        .querySelector('.nav-item svg')!
        .getBoundingClientRect();
      const styles = window.getComputedStyle(
        document.querySelector('.sidebar-toggle')!
      );
      const sidebarStyles = window.getComputedStyle(
        document.querySelector('.sidebar')!
      );
      const footerStyles = window.getComputedStyle(
        document.querySelector('.sidebar-footer')!
      );
      const icon = document.querySelector('.sidebar-toggle svg')!;
      const iconRect = icon.getBoundingClientRect();
      return {
        edge,
        center: control.left + control.width / 2,
        upperTipX: iconRect.left + (15 / 24) * iconRect.width,
        lowerTipX: iconRect.left + (15 / 24) * iconRect.width,
        pointX: iconRect.left + (9 / 24) * iconRect.width,
        workspaceCenter: workspace.left + workspace.width / 2,
        sidebarCenter:
          document.querySelector('.sidebar')!.getBoundingClientRect().left +
          document.querySelector('.sidebar')!.getBoundingClientRect().width / 2,
        workspaceBottom: workspace.bottom,
        titlebarBottom: titlebar.bottom,
        firstItemTop: firstItem.top,
        firstIconTop: firstIcon.top,
        toggleColor: styles.color,
        dividerColor: sidebarStyles.borderRightColor,
        strokeWidth: icon.getAttribute('stroke-width'),
        nameRight: name.right,
        footerRight: footer.right,
        background: styles.backgroundColor,
        footerPaddingRight: footerStyles.paddingRight,
        path: icon.querySelector('path')?.getAttribute('d')
      };
    });
    expect(Math.abs(expanded.center - expanded.edge)).toBeLessThan(1);
    expect(Math.abs(expanded.upperTipX - expanded.edge)).toBeLessThan(1);
    expect(Math.abs(expanded.lowerTipX - expanded.edge)).toBeLessThan(1);
    expect(expanded.pointX).toBeLessThan(expanded.edge);
    expect(
      Math.abs(expanded.workspaceCenter - expanded.sidebarCenter)
    ).toBeLessThan(1);
    expect(
      Math.abs(expanded.workspaceBottom - expanded.titlebarBottom)
    ).toBeLessThan(1);
    expect(expanded.firstItemTop - expanded.workspaceBottom).toBe(9);
    expect(expanded.toggleColor).toBe(expanded.dividerColor);
    expect(expanded.strokeWidth).toBe('1');
    expect(expanded.path).toBe('m15 18-6-6 6-6');
    expect(expanded.nameRight).toBeLessThanOrEqual(expanded.footerRight);
    expect(expanded.background).toBe('rgba(0, 0, 0, 0)');
    expect(expanded.footerPaddingRight).toBe('0px');

    await toggle.focus();
    await page.keyboard.press('Enter');
    const expand = page.getByRole('button', { name: '展开导航栏' });
    await expect(expand).toHaveAttribute('aria-expanded', 'false');
    await expect(sidebar).toHaveClass(/sidebar-compact/);
    const collapsed = await page.evaluate(() => {
      const edge = document
        .querySelector('.sidebar')!
        .getBoundingClientRect().right;
      const icon = document.querySelector('.sidebar-toggle svg')!;
      const rect = icon.getBoundingClientRect();
      const firstIcon = document
        .querySelector('.nav-item svg')!
        .getBoundingClientRect();
      return {
        edge,
        upperTipX: rect.left + (9 / 24) * rect.width,
        lowerTipX: rect.left + (9 / 24) * rect.width,
        pointX: rect.left + (15 / 24) * rect.width,
        firstIconTop: firstIcon.top,
        workspaceVisibility: window.getComputedStyle(
          document.querySelector('.workspace-label')!
        ).visibility,
        path: icon.querySelector('path')?.getAttribute('d')
      };
    });
    expect(Math.abs(collapsed.upperTipX - collapsed.edge)).toBeLessThan(1);
    expect(Math.abs(collapsed.lowerTipX - collapsed.edge)).toBeLessThan(1);
    expect(collapsed.pointX).toBeGreaterThan(collapsed.edge);
    expect(collapsed.path).toBe('m9 18 6-6-6-6');
    expect(collapsed.firstIconTop).toBe(expanded.firstIconTop);
    expect(collapsed.workspaceVisibility).toBe('hidden');
    await expand.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  test('shows a simulated fault in the main title bar without refreshing the session', async ({
    page
  }) => {
    await pageData(page);
    let mockRequests = 0;
    let refreshRequests = 0;
    await page.route('**/api/v1/mock/errors', (route) => {
      mockRequests += 1;
      return route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({
          error: {
            code: 'M0002',
            type: 'mock_1002',
            message: '模拟故障：访问凭据已过期，身份认证失败'
          }
        })
      });
    });
    await page.route('**/api/v1/auth/refresh', (route) => {
      refreshRequests += 1;
      return route.fulfill({ status: 204 });
    });
    await page.goto('./');
    const trigger = page.getByRole('button', {
      name: 'WORKSPACE',
      exact: true
    });
    await trigger.click();
    const message = page.locator('.topbar .topbar-message-slot');
    await expect(message.locator('.message-code')).toHaveText(['401', 'M0002']);
    await expect(message.locator('.message-type')).toHaveText('mock_1002');
    await expect(message).toContainText('访问凭据已过期，身份认证失败');
    expect(mockRequests).toBe(1);
    expect(refreshRequests).toBe(0);
    await expect(page.getByRole('heading', { name: '平台总览' })).toBeVisible();
    await page.getByRole('button', { name: '资源', exact: true }).click();
    await page.getByRole('button', { name: '新增资源', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: '基础配置', exact: true })
    ).toBeVisible();
    await trigger.focus();
    await page.keyboard.press('Enter');
    await expect(message.locator('.message-code')).toHaveText(['401', 'M0002']);
    expect(mockRequests).toBe(2);
    expect(refreshRequests).toBe(0);
    await expect(message).toBeHidden({ timeout: 7_000 });
  });

  test('logs in and restores the workspace session', async ({ page }) => {
    let healthRequests = 0;
    await page.route('**/health/ready', (route) => {
      healthRequests += 1;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'ready',
          service: 'opsseek-api',
          version: 'test',
          commit: 'test',
          build_time: '2026-01-01T00:00:00Z',
          timestamp: '2026-01-01T00:00:00Z',
          checks: {}
        })
      });
    });
    await pageData(page, true);
    await page.goto('/');
    await expect(page.getByLabel('OpsSeek 智能诊断平台')).toBeVisible();
    await expect(page.getByRole('heading', { name: '欢迎回来' })).toBeVisible();
    expect(healthRequests).toBe(0);
    expect(
      await page
        .locator('.login-shell')
        .evaluate((node) => getComputedStyle(node).backgroundImage)
    ).toContain('login-background');
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
    await page.getByLabel('账号', { exact: true }).fill('admin@example.test');
    const passwordInput = page.getByLabel('密码', { exact: true });
    await passwordInput.fill('test-password');
    const showPassword = page.getByRole('button', { name: '显示密码' });
    await expect(showPassword.locator('svg')).toBeVisible();
    await showPassword.click();
    await expect(passwordInput).toHaveAttribute('type', 'text');
    const hidePassword = page.getByRole('button', { name: '隐藏密码' });
    await expect(hidePassword.locator('svg')).toBeVisible();
    await hidePassword.click();
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page.getByRole('heading', { name: '平台总览' })).toBeVisible();
    await expect.poll(() => healthRequests).toBe(1);
  });

  test('switches scope and renders resources on desktop and mobile', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '项目' }).click();
    await page.getByRole('button', { name: /平台工程/ }).click();
    await page.getByLabel('切换项目').selectOption('project-1');
    await page.getByRole('button', { name: '资源' }).click();
    await expect(
      page.getByRole('heading', { name: '资源管理' }).first()
    ).toBeVisible();
    await page.getByRole('button', { name: '添加资源' }).click();
    await expect(page.getByRole('heading', { name: '基础配置' })).toBeVisible();
    await expect(page.locator('.resource-list-panel')).toBeHidden();
    await page.getByRole('button', { name: '取消' }).click();
    await expect(page.locator('.resource-list-panel')).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.getByRole('button', { name: '资源', exact: true })
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
  });

  test('shows team members, projects, user roles and permissions', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开权限菜单' }).click();
    await page.getByRole('button', { name: '团队管理' }).click();
    await expect(
      page.getByRole('heading', { name: '团队管理', level: 1 })
    ).toBeVisible();
    await page
      .locator('.access-team-trigger')
      .filter({ hasText: '平台工程' })
      .click();
    await expect(
      page.getByText('支付负责人', { exact: true }).first()
    ).toBeVisible();
    await expect(
      page.getByRole('article').getByText('支付服务', { exact: true })
    ).toBeVisible();
    await page.getByRole('button', { name: '用户管理' }).click();
    await expect(
      page.getByText('TeamOwner', { exact: true }).first()
    ).toBeVisible();
    await expect(
      page.getByText('member:grant', { exact: true }).first()
    ).toBeVisible();
    await page.getByRole('button', { name: '添加用户' }).click();
    await expect(page.getByRole('dialog', { name: '新增用户' })).toBeVisible();
    await page.getByLabel('授权级别').selectOption('team');
    await page.getByLabel('授权对象').selectOption(ids.team);
    await page.getByLabel('角色').selectOption('role-team-owner');
    await expect(page.getByLabel('角色')).toHaveValue('role-team-owner');
    await page.getByRole('button', { name: '关闭' }).click();

    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
  });

  test('hides team and project selectors for a platform administrator', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');

    await expect(page.getByLabel('切换项目')).toHaveCount(0);
    await expect(page.getByLabel('打开用户菜单')).toBeVisible();
  });

  test('opens the personal center and saves user display preferences', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');

    await page.getByLabel('打开用户菜单').click();
    await expect(page.getByRole('menu')).toBeVisible();
    await page.getByRole('heading', { name: '平台总览' }).click();
    await expect(page.getByRole('menu')).toHaveCount(0);
    await page.getByLabel('打开用户菜单').click();
    await page.getByRole('menuitem', { name: '个人中心' }).click();
    await expect(page.getByRole('heading', { name: '个人中心' })).toBeVisible();
    await page.getByLabel('显示名').fill('值守管理员');
    await page.getByRole('radio', { name: '深色' }).click();
    await page.getByRole('radio', { name: '窄栏悬浮展开' }).click();
    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.getByText('个人中心配置已保存')).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect
      .poll(() =>
        page
          .locator('html')
          .evaluate((element) =>
            getComputedStyle(element).getPropertyValue('--color-primary').trim()
          )
      )
      .toBe('#18a27d');

    await page.getByLabel('展开导航栏').hover();
    await expect(page.getByRole('button', { name: '总览' })).toBeVisible();
  });

  test('uses the selected dark theme for navigation and management surfaces', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByLabel('打开用户菜单').click();
    await page.getByRole('menuitem', { name: '个人中心' }).click();
    const preferenceUpdate = page.waitForRequest(
      (request) =>
        request.url().includes('/auth/me/preferences') &&
        request.method() === 'PUT'
    );
    await page.getByRole('radio', { name: '深色' }).click();
    await preferenceUpdate;
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    await page.reload();
    await expect(page.getByRole('heading', { name: '平台总览' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await page.getByLabel('打开用户菜单').click();
    await page.getByRole('menuitem', { name: '个人中心' }).click();

    await expect(page.getByRole('radio', { name: '深色' })).toHaveCSS(
      'background-color',
      'rgb(23, 62, 53)'
    );
    await expect(page.locator('.profile-panel').first()).toHaveCSS(
      'background-color',
      'rgb(42, 38, 35)'
    );
  });

  test('creates a team with a searchable icon picker', async ({ page }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开权限菜单' }).click();
    await page.getByRole('button', { name: '团队管理' }).click();
    await page.getByRole('button', { name: '添加团队' }).click();

    const dialog = page.getByRole('dialog', { name: '新增团队' });
    await expect(dialog.getByLabel('名称')).toBeVisible();
    await expect(dialog.getByLabel('团队名称')).toHaveCount(0);
    await dialog.getByLabel('选择团队图标').click();
    const picker = page.getByRole('dialog', { name: '选择图标' });
    await picker.getByLabel('搜索图标').fill('Activity');
    await picker.getByRole('button', { name: '选择图标 Activity' }).click();
    await dialog.getByLabel('选择团队图标').click();
    await picker.getByLabel('搜索图标').fill('Bot');
    await expect(
      picker.getByRole('button', { name: '选择图标 Bot', exact: true })
    ).toBeVisible();
    await picker
      .getByRole('button', { name: '选择图标 Bot', exact: true })
      .click();
    await dialog.getByLabel('名称').fill('数据库平台');
    await dialog.getByLabel('描述').fill('数据库平台团队');
    await dialog.getByRole('button', { name: '创建团队' }).click();
    await expect(page.getByText('团队“数据库平台”已创建')).toBeVisible();
  });

  test('creates a user with an auto-filled display name and inline password', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开权限菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByRole('button', { name: '添加用户' }).click();

    const dialog = page.getByRole('dialog', { name: '新增用户' });
    await dialog.getByLabel('用户名').fill('database-operator');
    await expect(dialog.getByLabel('显示名')).toHaveValue('database-operator');
    await expect(dialog.locator('.new-user-grant-header')).toContainText(
      '级别'
    );
    await expect(dialog.locator('.new-user-grant-header')).toContainText(
      '对象'
    );
    await expect(dialog.locator('.new-user-grant-header')).toContainText(
      '角色'
    );
    await dialog.getByLabel('授权级别').selectOption('team');
    await dialog.getByLabel('授权对象').selectOption(ids.team);
    await dialog.getByLabel('角色').selectOption('role-team-owner');
    await dialog.getByRole('button', { name: '创建用户并授权' }).click();
    await expect(dialog.locator('.created-credentials-inline')).toContainText(
      'GeneratedPassword123!'
    );
    await expect(
      dialog.locator('.created-credentials-inline')
    ).not.toContainText('database-operator');
    await expect(dialog.getByLabel('复制一次性密码')).toBeVisible();
  });

  test('offers same-scope resource grants for viewer roles', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开权限菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByRole('button', { name: '添加用户' }).click();

    const dialog = page.getByRole('dialog', { name: '新增用户' });
    await dialog.getByLabel('用户名').fill('project-viewer');
    await dialog.getByLabel('授权级别').selectOption('project');
    await dialog.getByLabel('授权对象').selectOption(ids.project);
    await dialog.getByLabel('角色').selectOption('role-project-viewer');
    await expect(
      dialog.getByText('项目观察员默认可读取该范围资源')
    ).toBeVisible();
    await expect(dialog.getByText('范围资源权限')).toBeVisible();
  });

  test('does not allow an administrator to edit a username', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开权限菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByLabel('编辑用户 支付负责人').click();

    const dialog = page.getByRole('dialog', { name: '编辑用户与授权' });
    await expect(dialog.getByLabel('用户名不可修改')).toBeDisabled();
    await expect(dialog.getByLabel('显示名')).toBeEditable();
  });

  test('keeps the orange primary color in light mode', async ({ page }) => {
    await pageData(page);
    await page.goto('/');

    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expect
      .poll(() =>
        page
          .locator('html')
          .evaluate((element) =>
            getComputedStyle(element).getPropertyValue('--color-primary').trim()
          )
      )
      .toBe('#e7601b');
  });
});

test.describe('T10 and T11 workbenches', () => {
  test('shows provider and Skill versions', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await expect(page.getByRole('button', { name: '大模型' })).toBeVisible();
    await page.getByRole('button', { name: '技能', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: '技能版本', exact: true })
    ).toBeVisible();
    await expect(
      page.locator('option').filter({ hasText: 'Default Diagnostic' })
    ).toHaveCount(1);
  });

  test('opens the independent Persona management page', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '专家' }).click();
    await expect(
      page.getByRole('heading', { name: '专家配置', exact: true })
    ).toBeVisible();
    await expect(
      page.getByRole('heading', { name: '创建专家版本', exact: true })
    ).toBeVisible();
  });

  test('restores a diagnosis session and its SSE-backed workbench', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '诊断' }).click();
    await expect(page.getByRole('heading', { name: '智能诊断' })).toBeVisible();
    await expect(
      page.getByRole('heading', { name: '支付服务诊断' })
    ).toBeVisible();
    await expect(page.getByText('已完成').first()).toBeVisible();
  });
});

test.describe('T13 inspection console', () => {
  test('creates a label policy and Webhook channel', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '巡检' }).click();
    const policyForm = page.locator('form').filter({ hasText: '标签选择器' });
    await policyForm.getByLabel('名称').fill('Hourly health');
    await policyForm.getByLabel('时区').fill('UTC');
    await policyForm
      .getByLabel('标签选择器（JSON 对象）')
      .fill('{"env":"prod"}');
    const personaSelect = policyForm.getByLabel('解释 Persona（可选）');
    await expect(
      personaSelect.locator('option', { hasText: 'Default Diagnostic' })
    ).toHaveCount(1);
    await personaSelect.selectOption('persona-1');
    await policyForm.getByRole('button', { name: '创建策略' }).click();
    await expect(page.getByText('Hourly health')).toBeVisible();

    const channelForm = page
      .locator('form')
      .filter({ hasText: 'HTTPS Webhook' });
    await channelForm.getByLabel('名称').fill('Incident webhook');
    await channelForm
      .getByLabel('HTTPS Webhook')
      .fill('https://hooks.example.test/incident');
    await channelForm.getByRole('button', { name: '添加渠道' }).click();
    await expect(page.getByText('Incident webhook')).toBeVisible();
  });
});

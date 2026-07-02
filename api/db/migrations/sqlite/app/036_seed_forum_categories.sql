INSERT INTO forum_categories (id, slug, name, description, sort_order, enabled)
VALUES
    ('forum-category-daily', 'daily', '日常', '日常交流与社区闲聊。', 10, 1),
    ('forum-category-tech', 'tech', '技术', '技术实践、开发工具与架构讨论。', 20, 1),
    ('forum-category-info', 'info', '情报', '资讯、线报与社区消息。', 30, 1),
    ('forum-category-review', 'review', '测评', '产品、服务与体验测评。', 40, 1),
    ('forum-category-trade', 'trade', '交易', '资源交换、买卖与需求撮合。', 50, 1),
    ('forum-category-carpool', 'carpool', '拼车', '订阅、服务与账号拼车。', 60, 1),
    ('forum-category-promotion', 'promotion', '推广', '项目、产品与优惠推广。', 70, 1),
    ('forum-category-life', 'life', '生活', '生活分享与非技术话题。', 80, 1),
    ('forum-category-dev', 'dev', 'Dev', '开发者项目、脚本与工具发布。', 90, 1),
    ('forum-category-image', 'image', '贴图', '图片、截图与视觉素材分享。', 100, 1),
    ('forum-category-exposure', 'exposure', '曝光', '风险提醒、避坑与争议记录。', 110, 1),
    ('forum-category-sandbox', 'sandbox', '沙盒', '测试、草稿与实验性讨论。', 120, 1)
ON CONFLICT(slug) DO UPDATE SET
    name = excluded.name,
    description = excluded.description,
    sort_order = excluded.sort_order,
    enabled = excluded.enabled;

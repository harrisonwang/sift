package config

// DefaultConfigExample is the starter configuration written by `sift config init`.
const DefaultConfigExample = `# sift 配置示例。
# 编辑此文件后，运行 ` + "`" + `sift config validate` + "`" + ` 校验配置。

# cache.db_path 可省略；省略时，sift 默认把 SQLite 缓存保存在
# $HOME/.sift/sift.db。
cache: {}

providers:
  # Twitter/X，通过 Nitter RSS 抓取，支持多个 Nitter 实例 fallback。
  - name: twitter
    enabled: true
    config:
      accounts: [msdev, OpenAI]
      # 可选；省略时会使用内置的公共默认实例。
      nitter_instances:
        - https://nitter.net
        - https://nitter.privacydev.net
        - https://nitter.poast.org
      max_items: 20
      # Nitter 账号 feed 包含转推和回复，并不只包含原创内容。
      # 默认：丢弃转推；include_replies 控制是否保留该账号自己的回复。
      include_retweets: false
      include_replies: false

  # Hacker News，通过官方 Firebase API 抓取。
  - name: hackernews
    enabled: true
    config:
      feeds: [top, show] # 可选：top、new、show、ask、best
      max_items: 50

  # Reddit 子版块，通过 RSS 抓取。
  - name: reddit
    enabled: true
    config:
      subreddits: [programming, MachineLearning]
      sort: hot # 可选：hot、new、top、rising、best
      limit: 30

  # 通用 RSS/Atom 博客和 feed。
  - name: rssblog
    enabled: true
    config:
      feeds:
        - url: https://openai.com/news/rss.xml
          source: openai_news
        - url: https://blog.google/products/gemini/rss/
          source: gemini_blog
        - url: https://deepmind.google/blog/rss.xml
          source: deepmind_blog
        # 注：Anthropic 暂无官方 RSS feed。
      max_items: 30
`

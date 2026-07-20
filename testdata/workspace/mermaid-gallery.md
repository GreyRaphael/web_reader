# Mermaid 图表验收

## Sequence

```mermaid
sequenceDiagram
  Browser->>Server: GET /api/fs/list
  Server-->>Browser: items
```

## Class

```mermaid
classDiagram
  class Reader
  Reader : +open(path)
```

## State

```mermaid
stateDiagram-v2
  [*] --> LoggedOut
  LoggedOut --> Reading: login
  Reading --> LoggedOut: logout
```

## ER

```mermaid
erDiagram
  WORKSPACE ||--o{ FILE : contains
```

## Journey

```mermaid
journey
  title 阅读文档
  section Reader
    登录: 5: User
    选择文件: 5: User
```

## Gantt

```mermaid
gantt
  title 首版计划
  dateFormat YYYY-MM-DD
  section Build
  Reader :done, 2026-07-20, 1d
```

## Pie

```mermaid
pie title 文件类型
  "Markdown" : 50
  "Text" : 30
  "Image" : 20
```

## Git graph

```mermaid
gitGraph
  commit
  branch feature
  commit
  checkout main
  merge feature
```

## Mindmap

```mermaid
mindmap
  root((Reader))
    Files
    Preview
    Outline
```

## Timeline

```mermaid
timeline
  title Web Reader
  Backend : Secure APIs
  Frontend : Responsive reader
```

## Quadrant

```mermaid
quadrantChart
  x-axis Low effort --> High effort
  y-axis Low value --> High value
  quadrant-1 Plan
  Reader shell: [0.4, 0.8]
```

## XY

```mermaid
xychart-beta
  x-axis [1, 2, 3]
  y-axis "Files" 0 --> 3
  line [1, 2, 3]
```

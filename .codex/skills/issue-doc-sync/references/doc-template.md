# Issue Doc Sync Template

Use this template when a closed issue needs a new lifecycle doc.

```md
# Title

> Type: `implemented`
> Updated: `YYYY-MM-DD`
> Summary: 一句话说明这次同步把哪个 closed issue 的最终实现结论沉淀进来。

## 背景

只保留当前实现仍有价值的背景，不抄录过程性讨论。

## 当前实现

描述已经落地的行为、边界和默认路径。

## 关键取舍

- 只保留对未来回看仍有意义的取舍

## 非目标 / 未覆盖

- 明确哪些内容不是当前实现的一部分

## 相关实现

- GitHub issue: `#NN`
- 相关代码或文档链接
```

Decision examples:

- `skip`
  - current docs already cover the durable result
- `merge`
  - update an existing canonical doc and record the target doc path
- `new-doc`
  - create a new lifecycle doc and update `docs/README.md`

import { CodeBlock } from "@/components/code-block"
import type { Root, Content, PhrasingContent } from "mdast"

function renderChildren(node: { children: unknown[] }): React.ReactNode {
  return (node.children as Content[]).map((child, i) => (
    <NodeRenderer key={i} node={child} />
  ))
}

function renderPhrasing(node: { children: unknown[] }): React.ReactNode {
  return (node.children as PhrasingContent[]).map((child, i) => (
    <NodeRenderer key={i} node={child} />
  ))
}

function NodeRenderer({ node }: { node: Content | Root }): React.ReactNode {
  switch (node.type) {
    case "heading": {
      const depth = node.depth
      const sizes: Record<number, string> = {
        1: "text-xl font-semibold mt-6 mb-2",
        2: "text-lg font-medium mt-5 mb-2",
        3: "text-base font-medium mt-4 mb-1.5",
        4: "text-sm font-normal mt-3 mb-1",
        5: "text-sm font-normal mt-2 mb-1",
        6: "text-xs font-normal mt-2 mb-1",
      }
      return (
        <h1 className={`text-white ${sizes[depth] || sizes[1]}`}>
          {renderPhrasing(node)}
        </h1>
      )
    }

    case "paragraph":
      return (
        <p className="text-white/80 leading-6 mb-2 last:mb-0">
          {renderPhrasing(node)}
        </p>
      )

    case "strong":
      return <strong className="font-semibold text-white">{renderPhrasing(node)}</strong>

    case "emphasis":
      return <em className="italic text-white/90">{renderPhrasing(node)}</em>

    case "delete":
      return <del className="line-through text-white/50">{renderPhrasing(node)}</del>

    case "inlineCode":
      return (
        <code className="px-1.5 py-0.5 mx-0.5 rounded-md bg-pink-500/10 text-pink-400 text-[13px] font-mono">
          {node.value}
        </code>
      )

    case "link":
      return (
        <a
          href={node.url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-primary underline underline-offset-2 hover:text-primary/80 transition-colors"
        >
          {renderPhrasing(node)}
        </a>
      )

    case "image":
      return (
        <img
          src={node.url}
          alt={node.alt || ""}
          className="my-2 max-w-full rounded-lg border border-white/10"
        />
      )

    case "list": {
      const Tag = node.ordered ? "ol" : "ul"
      const listStyle = node.ordered ? "list-decimal" : "list-disc"
      return (
        <Tag className={`${listStyle} ml-5 my-2 space-y-1`}>
          {node.children.map((item, i) => (
            <li key={i} className="text-white/80 leading-6 marker:text-white/40">
              {renderChildren(item)}
            </li>
          ))}
        </Tag>
      )
    }

    case "blockquote":
      return (
        <blockquote className="my-2 pl-4 border-l-2 border-primary/50 text-white/60 italic">
          {renderChildren(node)}
        </blockquote>
      )

    case "code":
      return (
        <CodeBlock language={node.lang || undefined}>
          {node.value}
        </CodeBlock>
      )

    case "table":
      return (
        <div className="my-3 overflow-x-auto rounded-xl border border-white/10">
          <table className="w-full text-sm">
            {node.children.map((row, ri) => {
              const isHeader = ri === 0
              const CellTag = isHeader ? "th" : "td"
              return (
                <tr key={ri} className={isHeader ? "border-b border-white/10" : ""}>
                  {row.children.map((cell, ci) => (
                    <CellTag
                      key={ci}
                      className={`px-3 py-2 text-left ${
                        isHeader
                          ? "text-white/70 font-medium bg-white/5"
                          : "text-white/80"
                      }`}
                    >
                      {renderPhrasing(cell)}
                    </CellTag>
                  ))}
                </tr>
              )
            })}
          </table>
        </div>
      )

    case "thematicBreak":
      return <hr className="my-4 border-t border-white/10" />

    case "html":
      return null

    case "text":
      return node.value

    case "break":
      return <br />

    case "root":
      return renderChildren(node)

    default:
      return null
  }
}

export function renderMdast(nodes: (Content | Root)[]): React.ReactNode {
  return nodes.map((node, i) => <NodeRenderer key={i} node={node} />)
}

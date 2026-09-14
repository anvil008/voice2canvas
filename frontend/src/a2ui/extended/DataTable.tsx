/**
 * DataTable component with positional columns and alignment from the
 * extended catalog.
 */
import type { CSSProperties } from "react";

const thStyle: CSSProperties = {
  color: "var(--fnt)",
  borderBottom: "1px solid var(--bd)",
  font: "500 11px/1 var(--font-sans)",
  letterSpacing: ".04em",
  padding: "10px 12px",
  textTransform: "uppercase",
  whiteSpace: "nowrap",
};

const tdStyle: CSSProperties = {
  color: "var(--tx)",
  borderBottom: "1px solid var(--bd)",
  font: "400 13px/1.4 var(--font-sans)",
  padding: "10px 12px",
};

export function DataTable({
  columns,
  rows,
  align = [],
}: {
  columns: string[];
  rows: Array<Array<string | number>>;
  align?: Array<"left" | "right">;
}) {
  if (!Array.isArray(columns) || columns.length === 0) {
    return <span className="extended-empty">No table data</span>;
  }
  return (
    <div className="extended-data-table">
      <table>
        <thead>
          <tr>
            {columns.map((column, index) => (
              <th
                key={column}
                style={{ ...thStyle, textAlign: align[index] ?? "left" }}
                title={column}
              >
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {(Array.isArray(rows) ? rows : []).map((row, rowIndex) => (
            <tr key={rowIndex}>
              {columns.map((_, columnIndex) => (
                <td
                  key={columnIndex}
                  style={{
                    ...tdStyle,
                    textAlign: align[columnIndex] ?? "left",
                    ...(typeof row[columnIndex] === "number" ? { fontFamily: "var(--font-mono)" } : {}),
                  }}
                  title={String(row[columnIndex] ?? "")}
                >
                  {row[columnIndex] ?? ""}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

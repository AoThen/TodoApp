import React from 'react';

interface PaginationProps {
  page: number;
  pages: number;
  total: number;
  onChange: (page: number) => void;
}

const Pagination: React.FC<PaginationProps> = ({ page, pages, total, onChange }) => {
  if (total <= 0 || pages <= 1) return null;

  const windowSize = 5;
  let start = Math.max(1, page - 2);
  const end = Math.min(pages, start + windowSize - 1);
  start = Math.max(1, end - windowSize + 1);

  const nums: number[] = [];
  for (let i = start; i <= end; i++) nums.push(i);

  return (
    <div className="pagination">
      <span className="pagination-total">共 {total} 条</span>
      <button
        className="pagination-btn"
        onClick={() => onChange(page - 1)}
        disabled={page <= 1}
      >
        上一页
      </button>
      {nums.map((n) => (
        <button
          key={n}
          className={`pagination-btn ${n === page ? 'active' : ''}`}
          onClick={() => onChange(n)}
        >
          {n}
        </button>
      ))}
      <button
        className="pagination-btn"
        onClick={() => onChange(page + 1)}
        disabled={page >= pages}
      >
        下一页
      </button>
    </div>
  );
};

export default Pagination;

const docsTree = () => {
  $('.tree-nav LI').click((e) => {
    if ( e.target.tagName === 'A' ) {
      e.stopPropagation();
      return;
    }

    const $tgt = $(e.target);
    const $li = $tgt.closest('LI')
    if ( $li.length ) {
      $li.toggleClass('open');
      e.stopPropagation();
    }
  });
}

$(document).ready(() => {
  docsTree();
});

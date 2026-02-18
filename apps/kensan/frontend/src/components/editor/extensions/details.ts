import { Node, mergeAttributes } from '@tiptap/core'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    details: {
      insertDetails: () => ReturnType
    }
  }
}

export const Details = Node.create({
  name: 'details',
  group: 'block',
  content: 'detailsSummary block+',
  defining: true,

  parseHTML() {
    return [{ tag: 'details' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['details', mergeAttributes(HTMLAttributes, { open: '' }), 0]
  },

  addCommands() {
    return {
      insertDetails:
        () =>
        ({ commands }) => {
          return commands.insertContent({
            type: 'details',
            content: [
              {
                type: 'detailsSummary',
                content: [{ type: 'text', text: 'セクションタイトル' }],
              },
              { type: 'paragraph' },
            ],
          })
        },
    }
  },
})

export const DetailsSummary = Node.create({
  name: 'detailsSummary',
  content: 'inline*',
  defining: true,

  parseHTML() {
    return [{ tag: 'summary' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['summary', mergeAttributes(HTMLAttributes), 0]
  },
})
